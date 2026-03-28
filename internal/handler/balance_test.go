package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"vtc-service/internal/model"
	"vtc-service/internal/store"
)

func TestBalanceDaily(t *testing.T) {
	fixedNow := parseTime("2026-03-28 14:00:00")
	withCurrentTime(t, fixedNow)

	s := store.New()
	s.AddTrips([]model.Trip{
		{DriverID: "d1", Timestamp: parseTime("2026-03-28 10:00:00"), Amount: 100.0},
	})

	h := &BalanceHandler{Store: s}

	req := httptest.NewRequest(http.MethodGet, "/balances?driver_id=d1&period=daily", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var balance map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&balance); err != nil {
		t.Fatal("invalid JSON response:", err)
	}

	// 100 gross → 15 commission → 85 net → 17 VAT, 17 Urssaf → 51 payout
	if balance["net_payout"] != 51.0 {
		t.Errorf("expected net_payout=51.0, got %v", balance["net_payout"])
	}
}

func TestBalanceMissingDriverID(t *testing.T) {
	s := store.New()
	h := &BalanceHandler{Store: s}

	req := httptest.NewRequest(http.MethodGet, "/balances?period=daily", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestBalanceInvalidPeriod(t *testing.T) {
	s := store.New()
	h := &BalanceHandler{Store: s}

	req := httptest.NewRequest(http.MethodGet, "/balances?driver_id=d1&period=yearly", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
