package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"vtc-service/internal/store"
)

// TestIngestJSON tests that uploading a valid JSON file stores the trips.
func TestIngestJSON(t *testing.T) {
	s := store.New()
	h := &IngestHandler{Store: s}

	jsonData := `[
		{"driver_id":"d1","timestamp":"2026-03-28T10:00:00Z","amount":50.0},
		{"driver_id":"d2","timestamp":"2026-03-28T11:00:00Z","amount":30.0}
	]`

	body, contentType := makeMultipartBody(t, "trips.json", []byte(jsonData))

	req := httptest.NewRequest(http.MethodPost, "/ingest", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal("response is not valid JSON:", err)
	}

	if got := resp["ingested"]; got != float64(2) {
		t.Errorf("expected ingested=2, got %v", got)
	}
	if s.Count() != 2 {
		t.Errorf("expected 2 trips in store, got %d", s.Count())
	}
}

func TestIngestCSV(t *testing.T) {
	s := store.New()
	h := &IngestHandler{Store: s}

	csvData := "driver_id,timestamp,amount\nd1,2026-03-28T10:00:00Z,50.0\n"
	body, contentType := makeMultipartBody(t, "trips.csv", []byte(csvData))

	req := httptest.NewRequest(http.MethodPost, "/ingest", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if s.Count() != 1 {
		t.Errorf("expected 1 trip, got %d", s.Count())
	}
}

func TestIngestCSV_ParsesNaiveTimestampInParis(t *testing.T) {
	s := store.New()
	h := &IngestHandler{Store: s}

	csvData := "driver_id,timestamp,amount\nd1,2026-03-28 10:00:00,50.0\n"
	body, contentType := makeMultipartBody(t, "trips.csv", []byte(csvData))

	req := httptest.NewRequest(http.MethodPost, "/ingest", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	trips := s.GetTrips("d1")
	if len(trips) != 1 {
		t.Fatalf("expected 1 trip, got %d", len(trips))
	}

	want := time.Date(2026, time.March, 28, 10, 0, 0, 0, france)
	if !trips[0].Timestamp.Equal(want) {
		t.Fatalf("expected timestamp %s, got %s", want, trips[0].Timestamp)
	}
}

func TestIngestJSON_ParsesNaiveTimestampInParis(t *testing.T) {
	s := store.New()
	h := &IngestHandler{Store: s}

	jsonData := `[{"driver_id":"d1","timestamp":"2026-03-28T10:00:00","amount":50.0}]`
	body, contentType := makeMultipartBody(t, "trips.json", []byte(jsonData))

	req := httptest.NewRequest(http.MethodPost, "/ingest", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	trips := s.GetTrips("d1")
	if len(trips) != 1 {
		t.Fatalf("expected 1 trip, got %d", len(trips))
	}

	want := time.Date(2026, time.March, 28, 10, 0, 0, 0, france)
	if !trips[0].Timestamp.Equal(want) {
		t.Fatalf("expected timestamp %s, got %s", want, trips[0].Timestamp)
	}
}

func TestIngestJSON_InvalidRecords(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "empty driver_id",
			data: `[{"driver_id":"","timestamp":"2026-03-28T10:00:00Z","amount":50.0}]`,
		},
		{
			name: "negative amount",
			data: `[{"driver_id":"d1","timestamp":"2026-03-28T10:00:00Z","amount":-10.0}]`,
		},
		{
			name: "missing timestamp",
			data: `[{"driver_id":"d1","amount":50.0}]`,
		},
		{
			name: "missing driver_id field",
			data: `[{"timestamp":"2026-03-28T10:00:00Z","amount":50.0}]`,
		},
		{
			name: "trailing content after array",
			data: `[{"driver_id":"d1","timestamp":"2026-03-28T10:00:00Z","amount":50.0}] garbage`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := store.New()
			h := &IngestHandler{Store: s}

			body, contentType := makeMultipartBody(t, "trips.json", []byte(tc.data))
			req := httptest.NewRequest(http.MethodPost, "/ingest", body)
			req.Header.Set("Content-Type", contentType)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
			if s.Count() != 0 {
				t.Errorf("expected 0 trips stored, got %d", s.Count())
			}
		})
	}
}

func TestIngest_PartialSuccess(t *testing.T) {
	s := store.New()
	h := &IngestHandler{Store: s}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	// valid JSON file
	fw, err := w.CreateFormFile("files", "trips.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte(`[{"driver_id":"d1","timestamp":"2026-03-28T10:00:00Z","amount":50.0}]`))

	// invalid file format
	fw, err = w.CreateFormFile("files", "trips.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("invalid content"))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on partial success, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal("response is not valid JSON:", err)
	}
	if got := resp["ingested"]; got != float64(1) {
		t.Errorf("expected ingested=1, got %v", got)
	}
	if _, hasWarnings := resp["warnings"]; !hasWarnings {
		t.Errorf("expected warnings in response for failed file")
	}
	if s.Count() != 1 {
		t.Errorf("expected 1 trip in store, got %d", s.Count())
	}
}

func TestIngestNoFiles(t *testing.T) {
	s := store.New()
	h := &IngestHandler{Store: s}

	// Send a multipart form but with no files
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// makeMultipartBody is a test helper that builds a multipart form with one file.
func makeMultipartBody(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("files", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(content)
	w.Close()
	return &body, w.FormDataContentType()
}
