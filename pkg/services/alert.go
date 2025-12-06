package services

import (
	"github.com/OpenFero/openfero/pkg/alertstore"
	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/models"
	"go.uber.org/zap"
)

// AlertBroadcastFunc is a callback function for broadcasting alert updates
type AlertBroadcastFunc func(entry models.AlertStoreEntry)

// alertBroadcaster is the function called when alerts are saved
var alertBroadcaster AlertBroadcastFunc

// SetAlertBroadcaster sets the function to be called when alerts are saved
func SetAlertBroadcaster(fn AlertBroadcastFunc) {
	alertBroadcaster = fn
}

// broadcastAlert calls the broadcaster if set
func broadcastAlert(alert models.Alert, status string, jobInfo *alertstore.JobInfo) {
	if alertBroadcaster != nil {
		entry := models.AlertStoreEntry{
			Alert:  alert,
			Status: status,
		}
		if jobInfo != nil {
			entry.JobInfo = &models.JobInfo{
				ConfigMapName: jobInfo.ConfigMapName,
				JobName:       jobInfo.JobName,
				Image:         jobInfo.Image,
			}
		}
		alertBroadcaster(entry)
	}
}

// CheckAlertStatus checks if alert status is valid
func CheckAlertStatus(status string) bool {
	return status == "resolved" || status == "firing"
}

// SaveAlert saves an alert to the alertstore
func SaveAlert(alertStore alertstore.Store, alert models.Alert, status string) {
	log.Debug("Saving alert in alert store",
		zap.String("alertname", alert.Labels["alertname"]),
		zap.String("status", status))

	// Convert to alertstore.Alert type
	storeAlert := alert.ToAlertStoreAlert()
	err := alertStore.SaveAlert(storeAlert, status)
	if err != nil {
		log.Error("Failed to save alert",
			zap.String("alertname", alert.Labels["alertname"]),
			zap.String("status", status),
			zap.Error(err))
		return
	}

	// Broadcast alert update to SSE clients
	broadcastAlert(alert, status, nil)
}

// SaveAlertWithJobInfo saves an alert to the alertstore with job information
func SaveAlertWithJobInfo(alertStore alertstore.Store, alert models.Alert, status string, jobInfo *alertstore.JobInfo) {
	log.Debug("Saving alert in alert store with job info",
		zap.String("alertname", alert.Labels["alertname"]),
		zap.String("status", status),
		zap.String("jobName", jobInfo.JobName))

	// Convert to alertstore.Alert type
	storeAlert := alert.ToAlertStoreAlert()
	err := alertStore.SaveAlertWithJobInfo(storeAlert, status, jobInfo)
	if err != nil {
		log.Error("Failed to save alert with job info",
			zap.String("alertname", alert.Labels["alertname"]),
			zap.String("status", status),
			zap.Error(err))
		return
	}

	// Broadcast alert update to SSE clients
	broadcastAlert(alert, status, jobInfo)
}
