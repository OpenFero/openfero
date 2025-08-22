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
	return s.SaveAlertWithJobInfo(alert, status, nil)
}

// SaveAlertWithJobInfo saves an alert to the in-memory store with job information
func (s *MemoryStore) SaveAlertWithJobInfo(alert alertstore.Alert, status string, jobInfo *alertstore.JobInfo) error {
	entry := alertstore.AlertEntry{
		Alert:     alert,
		Status:    status,
		Timestamp: time.Now(),
		JobInfo:   jobInfo,
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
		// Return all alerts in reverse chronological order (newest first), up to the limit
		result := make([]alertstore.AlertEntry, 0, len(s.alerts))

		// Copy the alerts in reverse order
		for i := len(s.alerts) - 1; i >= 0; i-- {
			result = append(result, s.alerts[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
		return result, nil
	}

	// Filter alerts based on query
	var results []alertstore.AlertEntry
	for i := len(s.alerts) - 1; i >= 0; i-- {
		if alertMatchesQuery(s.alerts[i], query) {
			results = append(results, s.alerts[i])
			if limit > 0 && len(results) >= limit {
				break
			}
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
	if strings.Contains(strings.ToLower(entry.Status), query) {
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

	// Check job info if present
	if entry.JobInfo != nil {
		if strings.Contains(strings.ToLower(entry.JobInfo.ConfigMapName), query) ||
			strings.Contains(strings.ToLower(entry.JobInfo.JobName), query) ||
			strings.Contains(strings.ToLower(entry.JobInfo.Image), query) {
			return true
		}
	}

	return false
}
