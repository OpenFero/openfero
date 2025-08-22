package alertstore

import (
	"time"
)

// AlertEntry represents a single alert in the store
type AlertEntry struct {
	Alert     Alert     `json:"alert"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	JobInfo   *JobInfo  `json:"jobInfo,omitempty"` // Information about triggered job (if any)
}

// Alert contains the alert information from Alertmanager
type Alert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    string            `json:"startsAt,omitempty"`
	EndsAt      string            `json:"endsAt,omitempty"`
}

// JobInfo contains information about a triggered job
type JobInfo struct {
	ConfigMapName string `json:"configMapName,omitempty"`
	JobName       string `json:"jobName,omitempty"`
	Image         string `json:"image,omitempty"`
}

// Store defines the interface for alert storage implementations
type Store interface {
	// SaveAlert saves an alert to the store
	SaveAlert(alert Alert, status string) error

	// SaveAlertWithJobInfo saves an alert to the store with job information
	SaveAlertWithJobInfo(alert Alert, status string, jobInfo *JobInfo) error

	// GetAlerts retrieves alerts, optionally filtered by query
	GetAlerts(query string, limit int) ([]AlertEntry, error)

	// Initialize prepares the store for use
	Initialize() error

	// Close cleans up any resources
	Close() error
}
