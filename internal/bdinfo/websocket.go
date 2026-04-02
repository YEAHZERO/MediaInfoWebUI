//go:build websocket

package bdinfo

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

type WebSocketMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type WebSocketHub struct {
	mu          sync.RWMutex
	connections map[*WebSocketConnection]bool
	register    chan *WebSocketConnection
	unregister  chan *WebSocketConnection
	broadcast   chan []byte
	jobs        func() []*Job
}

func NewWebSocketHub() *WebSocketHub {
	hub := &WebSocketHub{
		connections: make(map[*WebSocketConnection]bool),
		register:    make(chan *WebSocketConnection, 10),
		unregister:  make(chan *WebSocketConnection, 10),
		broadcast:   make(chan []byte, 100),
	}
	go hub.run()
	return hub
}

func (h *WebSocketHub) SetJobsProvider(fn func() []*Job) {
	h.jobs = fn
}

func (h *WebSocketHub) run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.connections[conn] = true
			h.mu.Unlock()
			if h.jobs != nil {
				jobs := h.jobs()
				for _, job := range jobs {
					msg := WebSocketMessage{Type: "job_update", Data: job}
					if data, err := json.Marshal(msg); err == nil {
						select {
						case conn.send <- data:
						default:
						}
					}
				}
			}

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.connections[conn]; ok {
				delete(h.connections, conn)
				close(conn.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for conn := range h.connections {
				select {
				case conn.send <- message:
				default:
					close(conn.send)
					delete(h.connections, conn)
				}
			}
			h.mu.RUnlock()

		case <-ticker.C:
			h.mu.RLock()
			count := len(h.connections)
			h.mu.RUnlock()
			if count > 0 {
				ping := WebSocketMessage{Type: "ping", Data: time.Now().Unix()}
				if data, err := json.Marshal(ping); err == nil {
					h.Broadcast(data)
				}
			}
		}
	}
}

func (h *WebSocketHub) Register(conn *WebSocketConnection) {
	h.register <- conn
}

func (h *WebSocketHub) Unregister(conn *WebSocketConnection) {
	h.unregister <- conn
}

func (h *WebSocketHub) Broadcast(message []byte) {
	select {
	case h.broadcast <- message:
	default:
		log.Printf("WebSocket broadcast channel full, dropping message")
	}
}

func (h *WebSocketHub) BroadcastJobUpdate(job *Job) {
	msg := WebSocketMessage{
		Type: "job_update",
		Data: job,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal job update: %v", err)
		return
	}
	h.Broadcast(data)
}

func (h *WebSocketHub) BroadcastProgress(jobID string, progress float64, etaSec int) {
	msg := WebSocketMessage{
		Type: "progress",
		Data: map[string]interface{}{
			"jobId":    jobID,
			"progress": progress,
			"etaSec":   etaSec,
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal progress: %v", err)
		return
	}
	h.Broadcast(data)
}

type WebSocketConnection struct {
	hub  *WebSocketHub
	send chan []byte
}

func NewWebSocketConnection(hub *WebSocketHub) *WebSocketConnection {
	return &WebSocketConnection{
		hub:  hub,
		send: make(chan []byte, 256),
	}
}

func (c *WebSocketConnection) SendChannel() chan []byte {
	return c.send
}
