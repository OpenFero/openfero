package memberlist

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/OpenFero/openfero/pkg/alertstore"
	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

const (
	// Default cluster name
	defaultClusterName = "openfero"
	// Channel buffer size for broadcasts
	broadcastQueueSize = 1024
)

// MemberlistStore implements the alertstore.Store interface using memberlist
type MemberlistStore struct {
	ml         *memberlist.Memberlist
	broadcasts *memberlist.TransmitLimitedQueue
	alerts     []alertEntry
	mutex      sync.RWMutex
	limit      int
	delegate   *delegate
}

// alertEntry represents a single alert in the store
type alertEntry struct {
	Alert     alertstore.Alert `json:"alert"`
	Status    string           `json:"status"`
	Timestamp time.Time        `json:"timestamp"`
}

// delegate is used to handle memberlist events and operations
type delegate struct {
	broadcasts *memberlist.TransmitLimitedQueue
	store      *MemberlistStore
}

// NewMemberlistStore creates a new memberlist-based alert store
func NewMemberlistStore(clustername string, limit int) *MemberlistStore {
	if clustername == "" {
		clustername = defaultClusterName
	}
	if limit <= 0 {
		limit = 100
	}

	store := &MemberlistStore{
		alerts: make([]alertEntry, 0, limit),
		limit:  limit,
	}

	// Create delegate
	store.delegate = &delegate{
		store: store,
	}

	return store
}

// Initialize sets up the memberlist cluster
func (s *MemberlistStore) Initialize() error {
	// Create memberlist config
	hostname, _ := os.Hostname()
	config := memberlist.DefaultLANConfig()
	config.Name = hostname
	config.Delegate = s.delegate
	config.Events = s.delegate

	// Create memberlist broadcast queue
	s.broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes:       func() int { return s.ml.NumMembers() },
		RetransmitMult: 3, // Retransmit a message up to 3 times
	}
	s.delegate.broadcasts = s.broadcasts

	// Create memberlist
	ml, err := memberlist.Create(config)
	if err != nil {
		return fmt.Errorf("failed to create memberlist: %w", err)
	}
	s.ml = ml

	// Try to join the cluster using Kubernetes DNS for service discovery
	// This assumes there's a headless service for the openfero pods
	serviceName := os.Getenv("MEMBERLIST_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "openfero-headless"
	}
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		// Try to get namespace from the mounted service account
		if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			namespace = string(data)
		} else {
			namespace = "default"
		}
	}

	// Form the service DNS name for Kubernetes
	serviceDns := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)
	log.Info("Trying to join memberlist cluster", zap.String("service", serviceDns))

	// Try joining the cluster
	_, err = s.ml.Join([]string{serviceDns})
	if err != nil {
		log.Warn("Failed to join cluster, creating new cluster", zap.Error(err))
		// This is not a fatal error - this node will form its own cluster
		// that others can join later
	}

	log.Info("Memberlist store initialized", zap.Int("members", s.ml.NumMembers()))
	return nil
}

// SaveAlert adds an alert to the store and broadcasts it to the cluster
func (s *MemberlistStore) SaveAlert(alert alertstore.Alert, status string) error {
	entry := alertEntry{
		Alert:     alert,
		Status:    status,
		Timestamp: time.Now(),
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Add alert to the front of the list (newest first)
	s.alerts = append([]alertEntry{entry}, s.alerts...)

	// Keep list at or under limit
	if len(s.alerts) > s.limit {
		s.alerts = s.alerts[:s.limit]
	}

	// Broadcast the new alert to other nodes
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	s.broadcasts.QueueBroadcast(&broadcast{
		msg:    data,
		notify: nil,
	})

	return nil
}

// GetAlerts retrieves alerts, optionally filtered by query
func (s *MemberlistStore) GetAlerts(query string, limit int) ([]alertstore.AlertEntry, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if limit <= 0 || limit > len(s.alerts) {
		limit = len(s.alerts)
	}

	// If no query, just return the alerts up to the limit
	if query == "" {
		result := make([]alertstore.AlertEntry, 0, limit)
		for i := 0; i < limit && i < len(s.alerts); i++ {
			result = append(result, alertstore.AlertEntry{
				Alert:     s.alerts[i].Alert,
				Status:    s.alerts[i].Status,
				Timestamp: s.alerts[i].Timestamp,
			})
		}
		return result, nil
	}

	// With query, filter the alerts
	result := make([]alertstore.AlertEntry, 0)
	query = strings.ToLower(query)

	for _, entry := range s.alerts {
		if s.alertMatchesQuery(entry, query) {
			result = append(result, alertstore.AlertEntry{
				Alert:     entry.Alert,
				Status:    entry.Status,
				Timestamp: entry.Timestamp,
			})
		}
		if len(result) >= limit {
			break
		}
	}

	return result, nil
}

// Close leaves the memberlist cluster
func (s *MemberlistStore) Close() error {
	if s.ml != nil {
		return s.ml.Leave(time.Second * 5)
	}
	return nil
}

// alertMatchesQuery checks if an alert matches the search query
func (s *MemberlistStore) alertMatchesQuery(entry alertEntry, query string) bool {
	// Check status
	if strings.Contains(strings.ToLower(entry.Status), query) {
		return true
	}

	// Check alertname
	if alertname, ok := entry.Alert.Labels["alertname"]; ok {
		if strings.Contains(strings.ToLower(alertname), query) {
			return true
		}
	}

	// Check labels
	for _, v := range entry.Alert.Labels {
		if strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}

	// Check annotations
	for _, v := range entry.Alert.Annotations {
		if strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}

	return false
}

// NodeMeta is used to retrieve meta-data about the current node
func (d *delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

// NotifyMsg is called when a user-data message is received
func (d *delegate) NotifyMsg(data []byte) {
	if len(data) == 0 {
		return
	}

	// Deserialize the alert entry
	var entry alertEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		log.Error("Failed to unmarshal alert", zap.Error(err))
		return
	}

	// Add the alert to our local store
	d.store.mutex.Lock()
	defer d.store.mutex.Unlock()

	// Check if this alert already exists (exact match by alertname, labels, and timestamp)
	for _, existing := range d.store.alerts {
		if existing.Timestamp.Equal(entry.Timestamp) {
			// Compare alertname if it exists
			if alertname, ok := existing.Alert.Labels["alertname"]; ok {
				if newAlertname, ok2 := entry.Alert.Labels["alertname"]; ok2 && alertname == newAlertname {
					// Already have this alert, skip it
					return
				}
			}
		}
	}

	// Add the alert to the front of our list
	d.store.alerts = append([]alertEntry{entry}, d.store.alerts...)

	// Keep list at or under limit
	if len(d.store.alerts) > d.store.limit {
		d.store.alerts = d.store.alerts[:d.store.limit]
	}
}

// GetBroadcasts is called when user data broadcasts are needed
func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return d.broadcasts.GetBroadcasts(overhead, limit)
}

// LocalState is used for a local node to send its full state to another node
func (d *delegate) LocalState(join bool) []byte {
	d.store.mutex.RLock()
	defer d.store.mutex.RUnlock()

	data, err := json.Marshal(d.store.alerts)
	if err != nil {
		log.Error("Failed to marshal local state", zap.Error(err))
		return []byte{}
	}
	return data
}

// MergeRemoteState is invoked when a remote node shares its local state
func (d *delegate) MergeRemoteState(buf []byte, join bool) {
	if len(buf) == 0 {
		return
	}

	var remoteAlerts []alertEntry
	if err := json.Unmarshal(buf, &remoteAlerts); err != nil {
		log.Error("Failed to unmarshal remote state", zap.Error(err))
		return
	}

	d.store.mutex.Lock()
	defer d.store.mutex.Unlock()

	// Add remote alerts to our list, avoiding duplicates
	for _, remoteEntry := range remoteAlerts {
		found := false
		for _, localEntry := range d.store.alerts {
			if localEntry.Timestamp.Equal(remoteEntry.Timestamp) {
				// Compare alertname if it exists
				if alertname, ok := localEntry.Alert.Labels["alertname"]; ok {
					if remoteAlertname, ok2 := remoteEntry.Alert.Labels["alertname"]; ok2 && alertname == remoteAlertname {
						found = true
						break
					}
				}
			}
		}

		if !found {
			d.store.alerts = append(d.store.alerts, remoteEntry)
		}
	}

	// Sort by timestamp (newest first)
	// Using simple bubble sort for simplicity
	for i := 0; i < len(d.store.alerts); i++ {
		for j := i + 1; j < len(d.store.alerts); j++ {
			if d.store.alerts[i].Timestamp.Before(d.store.alerts[j].Timestamp) {
				d.store.alerts[i], d.store.alerts[j] = d.store.alerts[j], d.store.alerts[i]
			}
		}
	}

	// Trim to limit
	if len(d.store.alerts) > d.store.limit {
		d.store.alerts = d.store.alerts[:d.store.limit]
	}
}

// NotifyJoin is invoked when a node joins the cluster
func (d *delegate) NotifyJoin(node *memberlist.Node) {
	log.Info("Node joined the cluster", zap.String("node", node.Name))
}

// NotifyLeave is invoked when a node leaves the cluster
func (d *delegate) NotifyLeave(node *memberlist.Node) {
	log.Info("Node left the cluster", zap.String("node", node.Name))
}

// NotifyUpdate is invoked when a node's metadata is updated
func (d *delegate) NotifyUpdate(node *memberlist.Node) {
	// Nothing to do here
}

// broadcast implements the memberlist.Broadcast interface
type broadcast struct {
	msg    []byte
	notify chan<- struct{}
}

func (b *broadcast) Invalidates(other memberlist.Broadcast) bool {
	// Never invalidate - we want all alerts to propagate
	return false
}

func (b *broadcast) Message() []byte {
	return b.msg
}

func (b *broadcast) Finished() {
	if b.notify != nil {
		close(b.notify)
	}
}
