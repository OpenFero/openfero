package memberlist

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"sort"

	"github.com/OpenFero/openfero/pkg/alertstore"
	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
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
	Alert     alertstore.Alert    `json:"alert"`
	Status    string              `json:"status"`
	Timestamp time.Time           `json:"timestamp"`
	JobInfo   *alertstore.JobInfo `json:"jobInfo,omitempty"`
}

// delegate is used to handle memberlist events and operations
type delegate struct {
	broadcasts *memberlist.TransmitLimitedQueue
	store      *MemberlistStore
}

// NewMemberlistStore creates a new memberlist-based alert store
func NewMemberlistStore(clustername string, limit int) *MemberlistStore {
	if limit <= 0 {
		limit = 100
	}

	log.Debug("Creating new memberlist store",
		zap.String("clusterName", clustername),
		zap.Int("alertLimit", limit))

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

	log.Debug("Initializing memberlist with config",
		zap.String("hostname", hostname))

	// Create memberlist first
	ml, err := memberlist.Create(config)
	if err != nil {
		log.Error("Failed to create memberlist", zap.Error(err))
		return fmt.Errorf("failed to create memberlist: %w", err)
	}

	// Store the memberlist reference
	s.ml = ml

	// Now that we have a memberlist, create the broadcast queue
	s.broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes:       func() int { return s.ml.NumMembers() },
		RetransmitMult: 3, // Retransmit a message up to 3 times
	}
	s.delegate.broadcasts = s.broadcasts

	// Try to join the cluster using Kubernetes DNS for service discovery
	// This assumes there's a headless service for the openfero pods
	serviceName := os.Getenv("MEMBERLIST_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "openfero-headless"
	}
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		// Try to get namespace from the mounted service account
		// Fix shadowing of err variable
		namespaceData, readErr := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if readErr == nil {
			namespace = string(namespaceData)
			log.Debug("Discovered namespace from service account", zap.String("namespace", namespace))
		} else {
			namespace = "default"
			log.Debug("Using default namespace", zap.Error(readErr))
		}
	}

	// Form the service DNS name for Kubernetes
	serviceDNS := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)
	log.Info("Trying to join memberlist cluster",
		zap.String("service", serviceDNS),
		zap.String("namespace", namespace))

	// Try joining the cluster
	joinCount, err := s.ml.Join([]string{serviceDNS})
	if err != nil {
		log.Warn("Failed to join cluster, creating new cluster",
			zap.Error(err),
			zap.String("serviceDNS", serviceDNS))
		// This is not a fatal error - this node will form its own cluster
		// that others can join later
	} else {
		log.Info("Successfully joined cluster", zap.Int("nodesJoined", joinCount))
	}

	log.Info("Memberlist store initialized",
		zap.Int("members", s.ml.NumMembers()),
		zap.String("localNode", s.ml.LocalNode().Name))
	return nil
}

// SaveAlert adds an alert to the store and broadcasts it to the cluster
func (s *MemberlistStore) SaveAlert(alert alertstore.Alert, status string) error {
	return s.SaveAlertWithJobInfo(alert, status, nil)
}

// SaveAlertWithJobInfo adds an alert with job info to the store and broadcasts it to the cluster
func (s *MemberlistStore) SaveAlertWithJobInfo(alert alertstore.Alert, status string, jobInfo *alertstore.JobInfo) error {
	entry := alertEntry{
		Alert:     alert,
		Status:    status,
		Timestamp: time.Now(),
		JobInfo:   jobInfo,
	}

	alertName := alert.Labels["alertname"]
	log.Debug("Saving alert to memberlist store",
		zap.String("alertname", alertName),
		zap.String("status", status))

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Add alert to the front of the list (newest first)
	s.alerts = append([]alertEntry{entry}, s.alerts...)

	// Keep list at or under limit
	if len(s.alerts) > s.limit {
		s.alerts = s.alerts[:s.limit]
		log.Debug("Trimmed alerts to limit", zap.Int("limit", s.limit))
	}

	// Broadcast the new alert to other nodes if memberlist is initialized
	data, err := json.Marshal(entry)
	if err != nil {
		log.Error("Failed to marshal alert for broadcast",
			zap.Error(err),
			zap.String("alertname", alertName))
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	if s.broadcasts != nil && s.ml != nil {
		s.broadcasts.QueueBroadcast(&broadcast{
			msg:    data,
			notify: nil,
		})
		log.Debug("Broadcast alert to cluster",
			zap.String("alertname", alertName),
			zap.Int("clusterSize", s.ml.NumMembers()))
	} else {
		log.Debug("Skipping broadcast - memberlist not initialized",
			zap.String("alertname", alertName))
	}

	return nil
}

// GetAlerts retrieves alerts, optionally filtered by query
func (s *MemberlistStore) GetAlerts(query string, limit int) ([]alertstore.AlertEntry, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	log.Debug("Getting alerts from store",
		zap.String("query", query),
		zap.Int("requestedLimit", limit),
		zap.Int("totalAlerts", len(s.alerts)))

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
				JobInfo:   s.alerts[i].JobInfo,
			})
		}
		log.Debug("Returning alerts with no query filter", zap.Int("resultCount", len(result)))
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
				JobInfo:   entry.JobInfo,
			})
		}
		if len(result) >= limit {
			break
		}
	}

	log.Debug("Returning filtered alerts",
		zap.String("query", query),
		zap.Int("resultCount", len(result)))
	return result, nil
}

// Close leaves the memberlist cluster
func (s *MemberlistStore) Close() error {
	if s.ml != nil {
		log.Info("Leaving memberlist cluster",
			zap.String("node", s.ml.LocalNode().Name),
			zap.Int("clusterSize", s.ml.NumMembers()))
		return s.ml.Leave(time.Second * 5)
	}
	log.Debug("No memberlist to close")
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
		log.Error("Failed to unmarshal alert in NotifyMsg",
			zap.Error(err),
			zap.Int("dataLength", len(data)))
		return
	}

	alertName := entry.Alert.Labels["alertname"]
	log.Debug("Received alert notification from cluster",
		zap.String("alertname", alertName),
		zap.String("status", entry.Status),
		zap.Time("timestamp", entry.Timestamp))

	// Add the alert to our local store
	if d.store == nil {
		log.Error("Store is nil in NotifyMsg")
		return
	}

	d.store.mutex.Lock()
	defer d.store.mutex.Unlock()

	// Check if this alert already exists (exact match by alertname, labels, and timestamp)
	for _, existing := range d.store.alerts {
		if existing.Timestamp.Equal(entry.Timestamp) {
			// Compare alertname if it exists
			if alertname, ok := existing.Alert.Labels["alertname"]; ok {
				if newAlertname, ok2 := entry.Alert.Labels["alertname"]; ok2 && alertname == newAlertname {
					// Already have this alert, skip it
					log.Debug("Skipping duplicate alert",
						zap.String("alertname", alertname),
						zap.Time("timestamp", entry.Timestamp))
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
		log.Debug("Trimmed alerts to limit after receiving new alert",
			zap.Int("limit", d.store.limit))
	}
}

// GetBroadcasts is called when user data broadcasts are needed
func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	if d.broadcasts == nil {
		return [][]byte{}
	}
	return d.broadcasts.GetBroadcasts(overhead, limit)
}

// LocalState is used for a local node to send its full state to another node
func (d *delegate) LocalState(join bool) []byte {
	if d.store == nil {
		log.Warn("LocalState called but store is nil")
		return []byte{}
	}

	d.store.mutex.RLock()
	defer d.store.mutex.RUnlock()

	data, err := json.Marshal(d.store.alerts)
	if err != nil {
		log.Error("Failed to marshal local state",
			zap.Error(err),
			zap.Int("alertCount", len(d.store.alerts)),
			zap.Bool("joinOperation", join))
		return []byte{}
	}
	log.Debug("Sending local state to remote node",
		zap.Int("alertCount", len(d.store.alerts)),
		zap.Bool("joinOperation", join),
		zap.Int("dataBytes", len(data)))
	return data
}

// MergeRemoteState is invoked when a remote node shares its local state
func (d *delegate) MergeRemoteState(buf []byte, join bool) {
	if len(buf) == 0 {
		log.Debug("Received empty remote state buffer")
		return
	}

	if d.store == nil {
		log.Warn("MergeRemoteState called but store is nil")
		return
	}

	log.Debug("Merging remote state",
		zap.Int("bufferSize", len(buf)),
		zap.Bool("joinOperation", join))

	var remoteAlerts []alertEntry
	if err := json.Unmarshal(buf, &remoteAlerts); err != nil {
		log.Error("Failed to unmarshal remote state",
			zap.Error(err),
			zap.Int("bufferSize", len(buf)))
		return
	}

	log.Debug("Unmarshalled remote alerts",
		zap.Int("remoteAlertCount", len(remoteAlerts)))

	d.store.mutex.Lock()
	defer d.store.mutex.Unlock()

	// Add remote alerts to our list, avoiding duplicates
	newAlertCount := 0
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
			newAlertCount++
		}
	}

	if newAlertCount > 0 {
		log.Debug("Added new alerts from remote state",
			zap.Int("newAlerts", newAlertCount),
			zap.Int("totalAlerts", len(d.store.alerts)))
	}

	// Sort by timestamp (newest first)
	sort.Slice(d.store.alerts, func(i, j int) bool {
		return d.store.alerts[i].Timestamp.After(d.store.alerts[j].Timestamp)
	})

	// Trim to limit
	if len(d.store.alerts) > d.store.limit {
		oldLength := len(d.store.alerts)
		d.store.alerts = d.store.alerts[:d.store.limit]
		log.Debug("Trimmed alerts to limit after merge",
			zap.Int("limit", d.store.limit),
			zap.Int("removed", oldLength-d.store.limit))
	}
}

// NotifyJoin is invoked when a node joins the cluster
func (d *delegate) NotifyJoin(node *memberlist.Node) {
	// Safe access to cluster size
	var clusterSize int
	if d.store != nil && d.store.ml != nil {
		clusterSize = d.store.ml.NumMembers()
	}

	log.Info("Node joined the cluster",
		zap.String("node", node.Name),
		zap.String("address", node.Address()),
		zap.Uint8("state", uint8(node.State)),
		zap.Int("clusterSize", clusterSize))
}

// NotifyLeave is invoked when a node leaves the cluster
func (d *delegate) NotifyLeave(node *memberlist.Node) {
	// Safe access to cluster size
	var clusterSize int
	if d.store != nil && d.store.ml != nil {
		clusterSize = d.store.ml.NumMembers()
	}

	log.Info("Node left the cluster",
		zap.String("node", node.Name),
		zap.String("address", node.Address()),
		zap.Uint8("state", uint8(node.State)),
		zap.Int("clusterSize", clusterSize))
}

// NotifyUpdate is invoked when a node's metadata is updated
func (d *delegate) NotifyUpdate(node *memberlist.Node) {
	log.Debug("Node metadata updated",
		zap.String("node", node.Name),
		zap.String("address", node.Address()),
		zap.Uint8("state", uint8(node.State)))
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
