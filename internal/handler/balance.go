package handler

// Business assumption:
// The service assumes drivers operate in France only. Timestamps with an explicit timezone offset are
// stored as-is; naive timestamps (no offset) are interpreted as Europe/Paris at ingest.
// All period boundaries (daily, weekly, monthly) are computed in Europe/Paris.
//
// If the service were to expand to multiple countries, the right approach would be:
// - store all timestamps in UTC at ingest (add .UTC() in parseTimestamp)
// - attach a timezone or country to each driver
// - convert to the driver's local timezone only at query time when computing period boundaries

import (
	"encoding/json"
	"net/http"
	"time"
	"vtc-service/internal/calculator"
	"vtc-service/internal/model"
	"vtc-service/internal/store"
)

var france = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		panic("timezone Europe/Paris not available: " + err.Error())
	}
	return loc
}()

// currentTime is overridable in tests so period window checks stay deterministic.
var currentTime = func() time.Time {
	return time.Now().In(france)
}

// BalanceHandler handles GET /balances
type BalanceHandler struct {
	Store *store.Store
}

func (h *BalanceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	driverID := q.Get("driver_id")
	period := q.Get("period")

	if driverID == "" {
		http.Error(w, "missing query param: driver_id", http.StatusBadRequest)
		return
	}

	if period == "" {
		http.Error(w, `missing query param: period`, http.StatusBadRequest)
		return
	}
	validPeriods := map[string]bool{"daily": true, "weekly": true, "monthly": true}
	if !validPeriods[period] {
		http.Error(w, `invalid period: must be "daily", "weekly", or "monthly"`, http.StatusBadRequest)
		return
	}

	trips := h.Store.GetTrips(driverID)
	gross := sumTripsForPeriod(trips, period)
	balance := calculator.Calculate(driverID, period, gross)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(balance)
}

// sumTripsForPeriod sums amounts of trips within [periodStart, now].
// excludes future-dated trips that may have been ingested.
func sumTripsForPeriod(trips []model.Trip, period string) float64 {
	now := currentTime()
	start := periodStart(now, period)

	var total float64
	for _, trip := range trips {
		if !trip.Timestamp.Before(start) && !trip.Timestamp.After(now) {
			total += trip.Amount
		}
	}
	return total
}

// periodStart returns the beginning of the current daily/weekly/monthly window
func periodStart(now time.Time, period string) time.Time {
	switch period {
	case "daily":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, france)
	case "weekly":
		// Beginning of current ISO week (Monday midnight)
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7 in ISO
		}
		d := now.AddDate(0, 0, -(weekday - 1))
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, france)
	case "monthly":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, france)
	default:
		return now
	}
}
