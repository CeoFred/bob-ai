package sessions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"bob/internal/llm"
)

// Session represents a stateful conversation context with Bob.
type Session struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	CreatedAt    time.Time     `json:"created_at"`
	LastActivity time.Time     `json:"last_activity"`
	Messages     []llm.Message `json:"messages"`
	TaskIDs      []string      `json:"task_ids"`
}

// Manager handles active sessions.
type Manager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	storagePath string
}

func NewManager(dataDir string) *Manager {
	storagePath := filepath.Join(dataDir, "sessions.json")
	mgr := &Manager{
		sessions:    make(map[string]*Session),
		storagePath: storagePath,
	}
	_ = mgr.loadFromDisk()
	return mgr
}

// GetOrCreate retrieves an existing session or creates a new one.
func (m *Manager) GetOrCreate(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[id]; ok {
		s.LastActivity = time.Now()
		return s
	}

	if id == "" {
		id = fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}

	session := &Session{
		ID:           id,
		Title:        "New Session",
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Messages:     make([]llm.Message, 0),
		TaskIDs:      make([]string, 0),
	}

	m.sessions[id] = session
	_ = m.saveToDisk()
	return session
}

// Get retrieves a session by ID.
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// List returns all active sessions ordered by last activity.
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	return list
}

// AppendMessage appends a message to the session's history and persists.
func (m *Manager) AppendMessage(sessionID string, msg llm.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[sessionID]; ok {
		s.Messages = append(s.Messages, msg)
		s.LastActivity = time.Now()
		if s.Title == "New Session" && msg.Role == llm.RoleUser && msg.Content != "" {
			runes := []rune(msg.Content)
			if len(runes) > 30 {
				s.Title = string(runes[:30]) + "..."
			} else {
				s.Title = string(runes)
			}
		}
		_ = m.saveToDisk()
	}
}

// AssociateTask links a task ID with a session.
func (m *Manager) AssociateTask(sessionID string, taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[sessionID]; ok {
		s.TaskIDs = append(s.TaskIDs, taskID)
		s.LastActivity = time.Now()
		_ = m.saveToDisk()
	}
}

func (m *Manager) saveToDisk() error {
	data, err := json.MarshalIndent(m.sessions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.storagePath, data, 0600)
}

func (m *Manager) loadFromDisk() error {
	data, err := os.ReadFile(m.storagePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &m.sessions)
}
