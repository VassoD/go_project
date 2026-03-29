package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
	"vtc-service/internal/model"
	"vtc-service/internal/store"
)

// IngestHandler handles POST /ingest.
type IngestHandler struct {
	Store *store.Store
}

func (h *IngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form - allows multiple file uploads.
	// Max 32MB in memory (files larger than that spill to disk).
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, `no files provided; send files as multipart form field "files"`, http.StatusBadRequest)
		return
	}

	var allTrips []model.Trip
	var parseErrors []string

	for _, fileHeader := range files {
		trips, err := parseFile(fileHeader)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("%s: %v", fileHeader.Filename, err))
			continue
		}
		allTrips = append(allTrips, trips...)
	}

	if len(parseErrors) > 0 && len(allTrips) == 0 {
		http.Error(w, "all files failed to parse:\n"+strings.Join(parseErrors, "\n"), http.StatusBadRequest)
		return
	}

	h.Store.AddTrips(allTrips)

	// Write JSON response.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]any{
		"ingested": len(allTrips),
		"total":    h.Store.Count(),
	}
	if len(parseErrors) > 0 {
		response["warnings"] = parseErrors
	}
	_ = json.NewEncoder(w).Encode(response)
}

// parseFile opens and delegates to the right parser based on file extension.
func parseFile(fileHeader *multipart.FileHeader) ([]model.Trip, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("cannot open: %w", err)
	}
	defer file.Close()

	name := strings.ToLower(fileHeader.Filename)
	switch {
	case strings.HasSuffix(name, ".json"):
		return parseJSON(file)
	case strings.HasSuffix(name, ".csv"):
		return parseCSV(file)
	default:
		return nil, fmt.Errorf("unsupported format (use .json or .csv)")
	}
}

// parseJSON decodes a JSON array of trips into a raw intermediate struct (Timestamp as string)
// so timestamp parsing and validation go through parseTimestamp, same as CSV.
func parseJSON(r io.Reader) ([]model.Trip, error) {
	var raw []struct {
		DriverID  string  `json:"driver_id"`
		Timestamp string  `json:"timestamp"`
		Amount    float64 `json:"amount"`
	}
	dec := json.NewDecoder(r)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("invalid JSON: unexpected content after array")
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("JSON file contains no trips")
	}

	trips := make([]model.Trip, 0, len(raw))
	for i, record := range raw {
		if record.DriverID == "" {
			return nil, fmt.Errorf("record %d: driver_id cannot be empty", i+1)
		}
		if record.Amount < 0 {
			return nil, fmt.Errorf("record %d: amount cannot be negative", i+1)
		}
		if record.Timestamp == "" {
			return nil, fmt.Errorf("record %d: timestamp is missing", i+1)
		}
		timestamp, err := parseTimestamp(record.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("record %d: invalid timestamp %q", i+1, record.Timestamp)
		}
		trips = append(trips, model.Trip{
			DriverID:  record.DriverID,
			Timestamp: timestamp,
			Amount:    record.Amount,
		})
	}
	return trips, nil
}

// parseCSV parses a CSV file with headers: driver_id, timestamp, amount.
func parseCSV(r io.Reader) ([]model.Trip, error) {
	reader := csv.NewReader(r)

	// Read header row
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("cannot read CSV header: %w", err)
	}

	// Build a map of column name -> index for flexible column ordering.
	colIndex := make(map[string]int)
	for i, col := range headers {
		colIndex[strings.TrimSpace(strings.ToLower(col))] = i
	}

	required := []string{"driver_id", "timestamp", "amount"}
	for _, col := range required {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	var trips []model.Trip
	lineNum := 1
	for {
		record, err := reader.Read()
		lineNum++
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		amount, err := strconv.ParseFloat(strings.TrimSpace(record[colIndex["amount"]]), 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid amount %q", lineNum, record[colIndex["amount"]])
		}
		if amount < 0 {
			return nil, fmt.Errorf("line %d: amount cannot be negative", lineNum)
		}

		timestamp, err := parseTimestamp(strings.TrimSpace(record[colIndex["timestamp"]]))
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid timestamp %q: %w", lineNum, record[colIndex["timestamp"]], err)
		}

		driverID := strings.TrimSpace(record[colIndex["driver_id"]])
		if driverID == "" {
			return nil, fmt.Errorf("line %d: driver_id cannot be empty", lineNum)
		}

		trips = append(trips, model.Trip{
			DriverID:  driverID,
			Timestamp: timestamp,
			Amount:    amount,
		})
	}

	if len(trips) == 0 {
		return nil, fmt.Errorf("CSV file contains no data rows")
	}
	return trips, nil
}

// parseTimestamp tries multiple common timestamp formats since different platforms (Uber, Bolt, etc.)
// may export timestamps differently. Naive timestamps (no timezone) are interpreted in France local
// time to match balance calculations.
func parseTimestamp(s string) (time.Time, error) {
	formats := []struct {
		layout          string
		useFranceLocale bool
	}{
		{layout: time.RFC3339},                                 // 2006-01-02T15:04:05Z07:00
		{layout: "2006-01-02T15:04:05", useFranceLocale: true}, // without timezone
		{layout: "2006-01-02 15:04:05", useFranceLocale: true}, // space separator
		{layout: "2006-01-02", useFranceLocale: true},          // date only
	}
	for _, format := range formats {
		var (
			t   time.Time
			err error
		)
		if format.useFranceLocale {
			t, err = time.ParseInLocation(format.layout, s, france)
		} else {
			t, err = time.Parse(format.layout, s)
		}
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized format (try RFC3339: 2006-01-02T15:04:05Z)")
}
