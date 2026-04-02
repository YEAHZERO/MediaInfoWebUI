//go:build !websocket

package bdinfo

// WebSocketHub is a stub implementation when websocket is disabled
type WebSocketHub struct{}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{}
}

func (h *WebSocketHub) SetJobsProvider(fn func() []*Job) {}
func (h *WebSocketHub) Register(conn *WebSocketConnection) {}
func (h *WebSocketHub) Unregister(conn *WebSocketConnection) {}
func (h *WebSocketHub) Broadcast(message []byte) {}
func (h *WebSocketHub) BroadcastJobUpdate(job *Job) {}
func (h *WebSocketHub) BroadcastProgress(jobID string, progress float64, etaSec int) {}

// WebSocketConnection is a stub implementation when websocket is disabled
type WebSocketConnection struct{}

func NewWebSocketConnection(hub *WebSocketHub) *WebSocketConnection {
	return &WebSocketConnection{}
}

func (c *WebSocketConnection) SendChannel() chan []byte {
	return nil
}
