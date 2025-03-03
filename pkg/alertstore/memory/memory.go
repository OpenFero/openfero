package memory

import (
	"strings"
	"sync"
	"time"

	"github.com/OpenFero/openfero/pkg/alertstore"
	log "github.com/OpenFero/openfero/pkg/logging"
)

// MemoryStore implements alertstore.Store using in-memory storage
type MemoryStore struct {
	alerts  []alertstore.AlertEntry
	mutex   sync.RWMutex
	maxSize int
}

// NewMemoryStore creates a new in-memory alert store
func NewMemoryStore(maxSize int) *MemoryStore {
	return &MemoryStore{
		alerts:  make([]alertstore.AlertEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// Initialize prepares the store
func (s *MemoryStore) Initialize() error {
	return nil // Nothing to do for memory store
}

// Close cleans up resources
func (s *MemoryStore) Close() error {
	return nil // Nothing to do for memory store
}

// SaveAlert saves an alert to the in-memory store
func (s *MemoryStore) SaveAlert(alert alertstore.Alert, status string) error {
	entry := alertstore.AlertEntry{
		Alert:     alert,
		Status:    status,
		Timestamp: time.Now(),
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.alerts) < s.maxSize {
		s.alerts = append(s.alerts, entry)
	} else {
		log.Debug("Alert store is full, dropping oldest alert")
		copy(s.alerts, s.alerts[1:])
		s.alerts[len(s.alerts)-1] = entry
	}

	return nil
}

// GetAlerts retrieves alerts, optionally filtered by query
func (s *MemoryStore) GetAlerts(query string, limit int) ([]alertstore.AlertEntry, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if query == "" {
		// Return all alerts, up to the limit
		if limit > 0 && limit < len(s.alerts) {
			return s.alerts[len(s.alerts)-limit:], nil
		}
		return s.alerts, nil
	}

	// Filter alerts based on query
	var results []alertstore.AlertEntry
	for _, entry := range s.alerts {
		if alertMatchesQuery(entry, query) {
			results = append(results, entry)
		}
	}

	return results, nil
}

// alertMatchesQuery checks if an alert matches the search query
func alertMatchesQuery(entry alertstore.AlertEntry, query string) bool {
	query = strings.ToLower(query)

	// Check alertname
	if alertname, ok := entry.Alert.Labels["alertname"]; ok {
		if strings.Contains(strings.ToLower(alertname), query) {
			return true
		}
	}

	// Check status
	if strings.Contains(entry.Status, query) {
		return true
	}

	// Check labels
	for _, value := range entry.Alert.Labels {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}

	// Check annotations
	for _, value := range entry.Alert.Annotations {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}

	return false
}
