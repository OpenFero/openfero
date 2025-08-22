package models

import (
	"time"

	"github.com/OpenFero/openfero/pkg/alertstore"
)

// HookMessage received from Alertmanager
type HookMessage struct {
	// Version of the Alertmanager message
	Version string `json:"version"`
	// Key used to group alerts
	GroupKey string `json:"groupKey"`
	// Status of the alert group (firing/resolved)
	Status string `json:"status" enum:"firing,resolved" example:"firing"`
	// Name of the receiver that handled the alert
	Receiver string `json:"receiver"`
	// Labels common to all alerts in the group
	GroupLabels map[string]string `json:"groupLabels"`
	// Labels common across all alerts
	CommonLabels map[string]string `json:"commonLabels"`
	// Annotations common across all alerts
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	// External URL to the Alertmanager
	ExternalURL string `json:"externalURL"`
	// List of alerts in the group
	Alerts []Alert `json:"alerts"`
}

// Alert information from Alertmanager
type Alert struct {
	// Key-value pairs of alert labels
	Labels map[string]string `json:"labels"`
	// Key-value pairs of alert annotations
	Annotations map[string]string `json:"annotations"`
	// Time when the alert started firing
	StartsAt string `json:"startsAt,omitempty"`
	// Time when the alert ended
	EndsAt string `json:"EndsAt,omitempty"`
}

// AlertStoreEntry represents a stored alert with status and timestamp
type AlertStoreEntry struct {
	Alert     Alert     `json:"alert"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	JobInfo   *JobInfo  `json:"jobInfo,omitempty"`
}

// JobInfo contains information about job definitions
type JobInfo struct {
	// Name of the ConfigMap containing the job definition
	ConfigMapName string `json:"configMapName"`
	// Name of the job
	JobName string `json:"jobName"`
	// Container image used by the job
	Image string `json:"image"`
}

// ToAlertStoreAlert converts an Alert to alertstore.Alert
func (a *Alert) ToAlertStoreAlert() alertstore.Alert {
	return alertstore.Alert{
		Labels:      a.Labels,
		Annotations: a.Annotations,
		StartsAt:    a.StartsAt,
		EndsAt:      a.EndsAt,
	}
}
