package handler

import (
	"bytes"
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
	var warnings []string

	for _, fileHeader := range files {
		trips, fileWarnings, err := parseFile(fileHeader)
		for _, warning := range fileWarnings {
			warnings = append(warnings, fmt.Sprintf("%s: %s", fileHeader.Filename, warning))
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", fileHeader.Filename, err))
		}
		allTrips = append(allTrips, trips...)
	}

	if len(allTrips) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ingested": 0,
			"total":    h.Store.Count(),
			"warnings": warnings,
		})
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
	if len(warnings) > 0 {
		response["warnings"] = warnings
	}
	_ = json.NewEncoder(w).Encode(response)
}

// parseFile opens and delegates to the right parser based on file extension.
func parseFile(fileHeader *multipart.FileHeader) ([]model.Trip, []string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open: %w", err)
	}
	defer file.Close()

	name := strings.ToLower(fileHeader.Filename)
	switch {
	case strings.HasSuffix(name, ".json"):
		return parseJSON(file)
	case strings.HasSuffix(name, ".csv"):
		return parseCSV(file)
	default:
		return nil, nil, fmt.Errorf("unsupported format (use .json or .csv)")
	}
}

// parseJSON decodes a JSON array of trips into a raw intermediate struct (Timestamp as string)
// so timestamp parsing and validation go through parseTimestamp, same as CSV.
func parseJSON(r io.Reader) ([]model.Trip, []string, error) {
	var raw []json.RawMessage
	dec := json.NewDecoder(r)
	if err := dec.Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if dec.More() {
		return nil, nil, fmt.Errorf("invalid JSON: unexpected content after array")
	}
	if len(raw) == 0 {
		return nil, nil, fmt.Errorf("JSON file contains no trips")
	}

	trips := make([]model.Trip, 0, len(raw))
	var warnings []string
	for i, blob := range raw {
		var record struct {
			DriverID  string   `json:"driver_id"`
			Timestamp string   `json:"timestamp"`
			Amount    *float64 `json:"amount"`
		}

		recordDec := json.NewDecoder(bytes.NewReader(blob))
		recordDec.DisallowUnknownFields()
		if err := recordDec.Decode(&record); err != nil {
			warnings = append(warnings, fmt.Sprintf("record %d: invalid JSON object: %v", i+1, err))
			continue
		}

		if record.DriverID == "" {
			warnings = append(warnings, fmt.Sprintf("record %d: driver_id cannot be empty", i+1))
			continue
		}
		if record.Amount == nil {
			warnings = append(warnings, fmt.Sprintf("record %d: amount is missing", i+1))
			continue
		}
		if *record.Amount < 0 {
			warnings = append(warnings, fmt.Sprintf("record %d: amount cannot be negative", i+1))
			continue
		}
		if record.Timestamp == "" {
			warnings = append(warnings, fmt.Sprintf("record %d: timestamp is missing", i+1))
			continue
		}
		timestamp, err := parseTimestamp(record.Timestamp)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("record %d: invalid timestamp %q", i+1, record.Timestamp))
			continue
		}
		trips = append(trips, model.Trip{
			DriverID:  record.DriverID,
			Timestamp: timestamp,
			Amount:    *record.Amount,
		})
	}

	if len(trips) == 0 {
		return nil, warnings, fmt.Errorf("JSON file contains no valid trips")
	}
	return trips, warnings, nil
}

// parseCSV parses a CSV file with headers: driver_id, timestamp, amount.
func parseCSV(r io.Reader) ([]model.Trip, []string, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1

	// Read header row
	headers, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read CSV header: %w", err)
	}

	// Build a map of column name -> index for flexible column ordering.
	colIndex := make(map[string]int)
	for i, col := range headers {
		colIndex[strings.TrimSpace(strings.ToLower(col))] = i
	}

	required := []string{"driver_id", "timestamp", "amount"}
	for _, col := range required {
		if _, ok := colIndex[col]; !ok {
			return nil, nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	var trips []model.Trip
	var warnings []string
	seenDataRow := false
	lineNum := 1
	for {
		record, err := reader.Read()
		lineNum++
		if err == io.EOF {
			break
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d: %v", lineNum, err))
			continue
		}
		if len(record) == 1 && strings.TrimSpace(record[0]) == "" {
			continue
		}
		seenDataRow = true

		driverIdx := colIndex["driver_id"]
		timestampIdx := colIndex["timestamp"]
		amountIdx := colIndex["amount"]
		if driverIdx >= len(record) || timestampIdx >= len(record) || amountIdx >= len(record) {
			warnings = append(warnings, fmt.Sprintf("line %d: missing one or more required values", lineNum))
			continue
		}

		driverID := strings.TrimSpace(record[driverIdx])
		if driverID == "" {
			warnings = append(warnings, fmt.Sprintf("line %d: driver_id cannot be empty", lineNum))
			continue
		}

		rawAmount := strings.TrimSpace(record[amountIdx])
		if rawAmount == "" {
			warnings = append(warnings, fmt.Sprintf("line %d: amount is missing", lineNum))
			continue
		}

		amount, err := strconv.ParseFloat(rawAmount, 64)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d: invalid amount %q", lineNum, record[amountIdx]))
			continue
		}
		if amount < 0 {
			warnings = append(warnings, fmt.Sprintf("line %d: amount cannot be negative", lineNum))
			continue
		}

		rawTimestamp := strings.TrimSpace(record[timestampIdx])
		if rawTimestamp == "" {
			warnings = append(warnings, fmt.Sprintf("line %d: timestamp is missing", lineNum))
			continue
		}

		timestamp, err := parseTimestamp(rawTimestamp)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d: invalid timestamp %q", lineNum, record[timestampIdx]))
			continue
		}

		trips = append(trips, model.Trip{
			DriverID:  driverID,
			Timestamp: timestamp,
			Amount:    amount,
		})
	}

	if !seenDataRow {
		return nil, nil, fmt.Errorf("CSV file contains no data rows")
	}
	if len(trips) == 0 {
		return nil, warnings, fmt.Errorf("CSV file contains no valid trips")
	}
	return trips, warnings, nil
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
