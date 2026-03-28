package model

import "time"

// Trip represents a single ride from any VTC platform.
type Trip struct {
	DriverID  string    `json:"driver_id"`
	Timestamp time.Time `json:"timestamp"`
	Amount    float64   `json:"amount"`
}
