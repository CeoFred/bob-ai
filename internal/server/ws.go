package server

import (
	"encoding/json"
	"net/http"
	"sync"

	"bob/internal/agent"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow Tailscale origins
	},
}

type ClientMessage struct {
	Action    string `json:"action"` // "create_task", "cancel_task", "approve_tool"
	SessionID string `json:"session_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	Prompt    string `json:"prompt,omitempty"`
	Approved  bool   `json:"approved,omitempty"`
}

type WSManager struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

func NewWSManager() *WSManager {
	return &WSManager{
		clients: make(map[*websocket.Conn]bool),
	}
}

func (wsm *WSManager) Broadcast(event agent.Event) {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	for conn := range wsm.clients {
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	s.wsManager.mu.Lock()
	s.wsManager.clients[conn] = true
	s.wsManager.mu.Unlock()

	defer func() {
		s.wsManager.mu.Lock()
		delete(s.wsManager.clients, conn)
		s.wsManager.mu.Unlock()
	}()

	// Send connection greeting
	_ = conn.WriteJSON(map[string]any{
		"type":    "connection.ready",
		"message": "Connected to Bob Agent API",
	})

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(msgBytes, &clientMsg); err != nil {
			_ = conn.WriteJSON(map[string]any{"error": "invalid message format"})
			continue
		}

		switch clientMsg.Action {
		case "create_task":
			if clientMsg.Prompt == "" {
				_ = conn.WriteJSON(map[string]any{"error": "prompt cannot be empty"})
				continue
			}
			task := s.agent.CreateTask(clientMsg.SessionID, clientMsg.Prompt)
			s.agent.Run(r.Context(), task)
			_ = conn.WriteJSON(map[string]any{
				"type":    "task.created",
				"task_id": task.ID,
			})

		case "cancel_task":
			if clientMsg.TaskID != "" {
				_ = s.agent.CancelTask(clientMsg.TaskID)
			}

		case "approve_tool":
			if clientMsg.TaskID != "" {
				_ = s.agent.ApproveTool(clientMsg.TaskID, clientMsg.Approved)
			}
		}
	}
}
