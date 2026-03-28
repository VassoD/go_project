package store

import (
	"sync"
	"vtc-service/internal/model"
)

// Store holds all ingested trips in memory.
// In production this would be replaced by a persistent store (PostgreSQL, SQLite).
// The RWMutex protects against race conditions - Go's HTTP server handles each
// request in its own goroutine, so concurrent reads/writes are possible.
type Store struct {
	mu    sync.RWMutex // multiple readers OR one writer at a time
	trips []model.Trip
}

// New creates and returns a pointer to a Store.
// We return *Store (a pointer) so callers share the same instance,
// not a copy. This is crucial — without the pointer, AddTrips would
// modify a copy and the original would never change.
func New() *Store {
	return &Store{}
}

// AddTrips appends a batch of trips to the store.
// Lock (not RLock) is used because we are writing
// only one goroutine may hold a write lock at a time, blocking all concurrent readers/writers.
func (s *Store) AddTrips(trips []model.Trip) {
	s.mu.Lock()
	defer s.mu.Unlock() // defer = "run this when the function exits"
	s.trips = append(s.trips, trips...)
}

// GetTrips returns all trips for a given driver.
// RLock allows concurrent reads (multiple goroutines can read simultaneously).
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
