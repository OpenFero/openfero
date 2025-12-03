package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/models"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for development - in production, restrict this
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// WSClient represents a WebSocket client connection
type WSClient struct {
	hub  *WSHub
	conn *websocket.Conn
	send chan []byte
}

// WSHub manages all WebSocket client connections
type WSHub struct {
	clients    map[*WSClient]struct{}
	broadcast  chan []byte
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex
}

// Global WebSocket hub instance
var wsHub *WSHub
var wsOnce sync.Once

// GetWSHub returns the singleton WebSocket hub instance
func GetWSHub() *WSHub {
	wsOnce.Do(func() {
		wsHub = &WSHub{
			clients:    make(map[*WSClient]struct{}),
			broadcast:  make(chan []byte, 256),
			register:   make(chan *WSClient),
			unregister: make(chan *WSClient),
		}
		go wsHub.run()
	})
	return wsHub
}

// run processes WebSocket hub events
func (h *WSHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()
			log.Debug("WebSocket client connected", zap.Int("totalClients", len(h.clients)))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Debug("WebSocket client disconnected", zap.Int("totalClients", len(h.clients)))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client buffer full, close connection
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected WebSocket clients
func (h *WSHub) Broadcast(msgType string, data interface{}) {
	msg := WSMessage{Type: msgType, Data: data}
	jsonData, err := json.Marshal(msg)
	if err != nil {
		log.Error("Failed to marshal WebSocket message", zap.Error(err))
		return
	}
	h.broadcast <- jsonData
}

// BroadcastAlertWS sends an alert update to all connected WebSocket clients
func BroadcastAlertWS(entry models.AlertStoreEntry) {
	GetWSHub().Broadcast("alert", entry)
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Debug("WebSocket read error", zap.Error(err))
			}
			break
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *WSClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current WebSocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// WebSocketHandler handles WebSocket connections
// @Summary WebSocket endpoint for real-time updates
// @Description Establish a WebSocket connection for real-time alert and job status updates
// @Tags websocket
// @Success 101 {string} string "Switching Protocols"
// @Router /api/ws [get]
func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	hub := GetWSHub()
	client := &WSClient{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
	}
	hub.register <- client

	// Send initial connected message
	connectedMsg := WSMessage{Type: "connected", Data: map[string]string{"message": "Connected to OpenFero WebSocket"}}
	jsonData, _ := json.Marshal(connectedMsg)
	client.send <- jsonData

	log.Debug("WebSocket client connected",
		zap.String("remoteAddr", r.RemoteAddr))

	// Start read and write pumps in goroutines
	go client.writePump()
	go client.readPump()
}
