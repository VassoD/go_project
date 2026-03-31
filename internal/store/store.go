package store

import (
	"sync"
	"vtc-service/internal/model"
)

// Store holds all ingested trips in memory, protected by a RWMutex for concurrent access.
// In production this would be replaced by a persistent store (PostgreSQL, SQLite).
type Store struct {
	mu    sync.RWMutex
	trips []model.Trip
}

// New creates and returns a new Store.
func New() *Store {
	return &Store{}
}

// AddTrips appends a batch of trips to the store.
func (s *Store) AddTrips(trips []model.Trip) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trips = append(s.trips, trips...)
}

// GetTrips returns all trips for a given driver.
func (s *Store) GetTrips(driverID string) []model.Trip {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []model.Trip
	for _, trip := range s.trips {
		if trip.DriverID == driverID {
			result = append(result, trip)
		}
	}
	return result
}

// Count returns the total number of stored trips.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.trips)
}
