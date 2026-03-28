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

func TestBalanceUnknownDriver(t *testing.T) {
	fixedNow := parseTime("2026-03-28 14:00:00")
	withCurrentTime(t, fixedNow)

	s := store.New()
	h := &BalanceHandler{Store: s}

	req := httptest.NewRequest(http.MethodGet, "/balances?driver_id=unknown&period=daily", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var balance map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&balance); err != nil {
		t.Fatal("invalid JSON response:", err)
	}

	if balance["gross_amount"] != 0.0 {
		t.Errorf("expected gross_amount=0.0 for unknown driver, got %v", balance["gross_amount"])
	}
	if balance["net_payout"] != 0.0 {
		t.Errorf("expected net_payout=0.0 for unknown driver, got %v", balance["net_payout"])
	}
}

func TestBalanceWeekly(t *testing.T) {
	// now = Wednesday 2026-03-25; ISO week starts Monday 2026-03-23
	fixedNow := parseTime("2026-03-25 12:00:00")
	withCurrentTime(t, fixedNow)

	s := store.New()
	s.AddTrips([]model.Trip{
		{DriverID: "d1", Timestamp: parseTime("2026-03-24 09:00:00"), Amount: 200.0}, // Tuesday – in window
		{DriverID: "d1", Timestamp: parseTime("2026-03-22 23:59:59"), Amount: 50.0},  // previous Sunday – out
	})

	h := &BalanceHandler{Store: s}
	req := httptest.NewRequest(http.MethodGet, "/balances?driver_id=d1&period=weekly", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var balance map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&balance); err != nil {
		t.Fatal("invalid JSON response:", err)
	}

	// 200 gross → 30 commission → 170 net → 34 VAT, 34 Urssaf → 102 payout
	if balance["gross_amount"] != 200.0 {
		t.Errorf("expected gross_amount=200.0, got %v", balance["gross_amount"])
	}
	if balance["net_payout"] != 102.0 {
		t.Errorf("expected net_payout=102.0, got %v", balance["net_payout"])
	}
}

func TestBalanceMonthly(t *testing.T) {
	// now = 2026-03-25; month window starts 2026-03-01
	fixedNow := parseTime("2026-03-25 12:00:00")
	withCurrentTime(t, fixedNow)

	s := store.New()
	s.AddTrips([]model.Trip{
		{DriverID: "d1", Timestamp: parseTime("2026-03-01 00:00:00"), Amount: 100.0}, // first of month – in
		{DriverID: "d1", Timestamp: parseTime("2026-03-20 08:00:00"), Amount: 100.0}, // mid-month – in
		{DriverID: "d1", Timestamp: parseTime("2026-02-28 23:59:59"), Amount: 50.0},  // last day of Feb – out
	})

	h := &BalanceHandler{Store: s}
	req := httptest.NewRequest(http.MethodGet, "/balances?driver_id=d1&period=monthly", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var balance map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&balance); err != nil {
		t.Fatal("invalid JSON response:", err)
	}

	// 200 gross → 30 commission → 170 net → 34 VAT, 34 Urssaf → 102 payout
	if balance["gross_amount"] != 200.0 {
		t.Errorf("expected gross_amount=200.0, got %v", balance["gross_amount"])
	}
	if balance["net_payout"] != 102.0 {
		t.Errorf("expected net_payout=102.0, got %v", balance["net_payout"])
	}
}
