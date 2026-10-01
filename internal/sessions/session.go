package sessions

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"bob/internal/llm"
)

// SessionType represents whether a session is scoped to a specific project or is a general PC assistant conversation.
type SessionType string

const (
	SessionTypeProject      SessionType = "project"
	SessionTypeConversation SessionType = "conversation"
)

// Project represents a registered workspace folder that contains multiple chats/threads.
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Summary   string    `json:"summary,omitempty"`
	TechStack []string  `json:"tech_stack,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProjectWithSessions combines a project with its active conversation threads.
type ProjectWithSessions struct {
	Project
	Sessions []*Session `json:"sessions"`
}

// Session represents a stateful conversation context with Bob.
type Session struct {
	ID           string        `json:"id"`
	ProjectID    string        `json:"project_id,omitempty"`   // Associated Project ID if project session
	Type         SessionType   `json:"type"`                   // "project" or "conversation"
	Title        string        `json:"title"`
	ProjectPath  string        `json:"project_path,omitempty"` // Absolute path to project root
	ProjectName  string        `json:"project_name,omitempty"` // Project name/basename
	CreatedAt    time.Time     `json:"created_at"`
	LastActivity time.Time     `json:"last_activity"`
	Messages     []llm.Message `json:"messages"`
	TaskIDs      []string      `json:"task_ids"`
}

// Manager handles active projects and sessions.
type Manager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	projects    map[string]*Project
	storagePath string
	projPath    string
}

// NewUUID generates a standard RFC 4122 version 4 UUID.
func NewUUID() string {
	var b [16]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func NewManager(dataDir string) *Manager {
	storagePath := filepath.Join(dataDir, "sessions.json")
	projPath := filepath.Join(dataDir, "projects.json")
	mgr := &Manager{
		sessions:    make(map[string]*Session),
		projects:    make(map[string]*Project),
		storagePath: storagePath,
		projPath:    projPath,
	}
	_ = mgr.loadFromDisk()
	return mgr
}

// GetOrCreateProject registers or retrieves a project for the given path and auto-analyzes its stack.
func (m *Manager) GetOrCreateProject(path, name string) *Project {
	m.mu.Lock()
	defer m.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}

	// Check if already registered
	for _, p := range m.projects {
		if p.Path == path {
			if name != "" && p.Name != name {
				p.Name = name
				p.UpdatedAt = time.Now()
				_ = m.saveToDisk()
			}
			return p
		}
	}

	if name == "" {
		name = filepath.Base(path)
	}

	summary, techStack := InspectProject(path)

	proj := &Project{
		ID:        fmt.Sprintf("proj_%s", NewUUID()),
		Name:      name,
		Path:      path,
		Summary:   summary,
		TechStack: techStack,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.projects[proj.ID] = proj
	_ = m.saveToDisk()
	return proj
}

// GetProject returns a project by ID.
func (m *Manager) GetProject(id string) (*Project, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.projects[id]
	return p, ok
}

// ListProjects returns all registered projects with their associated sessions.
func (m *Manager) ListProjects() []*ProjectWithSessions {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Group sessions by ProjectID or ProjectPath
	sessionsByProj := make(map[string][]*Session)
	for _, s := range m.sessions {
		if s.Type == SessionTypeProject {
			key := s.ProjectID
			if key == "" && s.ProjectPath != "" {
				// find matching project
				for _, p := range m.projects {
					if p.Path == s.ProjectPath {
						key = p.ID
						break
					}
				}
			}
			if key != "" {
				sessionsByProj[key] = append(sessionsByProj[key], s)
			}
		}
	}

	// Sort sessions in each project by last activity desc
	for _, slist := range sessionsByProj {
		sort.Slice(slist, func(i, j int) bool {
			return slist[i].LastActivity.After(slist[j].LastActivity)
		})
	}

	list := make([]*ProjectWithSessions, 0, len(m.projects))
	for _, p := range m.projects {
		slist := sessionsByProj[p.ID]
		if slist == nil {
			slist = make([]*Session, 0)
		}
		list = append(list, &ProjectWithSessions{
			Project:  *p,
			Sessions: slist,
		})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})

	return list
}

// CreateProjectSession creates a new chat conversation thread under a specific project.
func (m *Manager) CreateProjectSession(projectID, title string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	proj, ok := m.projects[projectID]
	if !ok {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	id := NewUUID()
	if title == "" {
		title = "New Thread"
	}

	session := &Session{
		ID:           id,
		ProjectID:    proj.ID,
		Type:         SessionTypeProject,
		Title:        title,
		ProjectPath:  proj.Path,
		ProjectName:  proj.Name,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Messages:     make([]llm.Message, 0),
		TaskIDs:      make([]string, 0),
	}

	proj.UpdatedAt = time.Now()
	m.sessions[id] = session
	_ = m.saveToDisk()
	return session, nil
}

// Create explicitly creates a new session of the given type and project path.
func (m *Manager) Create(sessType SessionType, projectPath, title string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := NewUUID()
	if sessType == "" {
		if projectPath != "" {
			sessType = SessionTypeProject
		} else {
			sessType = SessionTypeConversation
		}
	}

	projectID := ""
	projectName := ""
	if projectPath != "" {
		abs, err := filepath.Abs(projectPath)
		if err == nil {
			projectPath = abs
		}
		projectName = filepath.Base(projectPath)

		// Link with project entity
		for _, p := range m.projects {
			if p.Path == projectPath {
				projectID = p.ID
				projectName = p.Name
				p.UpdatedAt = time.Now()
				break
			}
		}
		if projectID == "" {
			summary, techStack := InspectProject(projectPath)
			proj := &Project{
				ID:        fmt.Sprintf("proj_%s", NewUUID()),
				Name:      projectName,
				Path:      projectPath,
				Summary:   summary,
				TechStack: techStack,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			m.projects[proj.ID] = proj
			projectID = proj.ID
		}
	}

	if title == "" {
		if sessType == SessionTypeProject && projectName != "" {
			title = projectName
		} else {
			title = "New Conversation"
		}
	}

	session := &Session{
		ID:           id,
		ProjectID:    projectID,
		Type:         sessType,
		Title:        title,
		ProjectPath:  projectPath,
		ProjectName:  projectName,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Messages:     make([]llm.Message, 0),
		TaskIDs:      make([]string, 0),
	}

	m.sessions[id] = session
	_ = m.saveToDisk()
	return session
}

// GetOrCreate retrieves an existing session or creates a new conversation session.
func (m *Manager) GetOrCreate(id string) *Session {
	return m.GetOrCreateWithOptions(id, SessionTypeConversation, "", "")
}

// GetOrCreateWithOptions retrieves an existing session or creates one with specific parameters.
func (m *Manager) GetOrCreateWithOptions(id string, sessType SessionType, projectPath, title string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id != "" {
		if s, ok := m.sessions[id]; ok {
			s.LastActivity = time.Now()
			return s
		}
	} else {
		id = NewUUID()
	}

	if sessType == "" {
		if projectPath != "" {
			sessType = SessionTypeProject
		} else {
			sessType = SessionTypeConversation
		}
	}

	projectID := ""
	projectName := ""
	if projectPath != "" {
		abs, err := filepath.Abs(projectPath)
		if err == nil {
			projectPath = abs
		}
		projectName = filepath.Base(projectPath)

		for _, p := range m.projects {
			if p.Path == projectPath {
				projectID = p.ID
				projectName = p.Name
				p.UpdatedAt = time.Now()
				break
			}
		}
		if projectID == "" {
			summary, techStack := InspectProject(projectPath)
			proj := &Project{
				ID:        fmt.Sprintf("proj_%s", NewUUID()),
				Name:      projectName,
				Path:      projectPath,
				Summary:   summary,
				TechStack: techStack,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			m.projects[proj.ID] = proj
			projectID = proj.ID
		}
	}

	if title == "" {
		if sessType == SessionTypeProject && projectName != "" {
			title = projectName
		} else {
			title = "New Conversation"
		}
	}

	session := &Session{
		ID:           id,
		ProjectID:    projectID,
		Type:         sessType,
		Title:        title,
		ProjectPath:  projectPath,
		ProjectName:  projectName,
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

// Delete removes a session from memory and disk.
func (m *Manager) Delete(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; ok {
		delete(m.sessions, id)
		_ = m.saveToDisk()
		return true
	}
	return false
}

// DeleteProject removes a project and its associated sessions from memory and disk.
func (m *Manager) DeleteProject(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.projects[id]
	if !ok {
		return false
	}

	delete(m.projects, id)
	for sid, s := range m.sessions {
		if s.ProjectID == id || (s.ProjectPath != "" && s.ProjectPath == p.Path) {
			delete(m.sessions, sid)
		}
	}
	_ = m.saveToDisk()
	return true
}

// List returns all active sessions ordered by last activity.
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].LastActivity.After(list[j].LastActivity)
	})

	return list
}

// AppendMessage appends a message to the session's history and persists.
func (m *Manager) AppendMessage(sessionID string, msg llm.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[sessionID]; ok {
		s.Messages = append(s.Messages, msg)
		s.LastActivity = time.Now()
		if (s.Title == "New Session" || s.Title == "New Conversation" || s.Title == "New Thread") && msg.Role == llm.RoleUser && msg.Content != "" {
			runes := []rune(msg.Content)
			if len(runes) > 30 {
				s.Title = string(runes[:30]) + "..."
			} else {
				s.Title = string(runes)
			}
		}
		if s.ProjectID != "" {
			if p, ok := m.projects[s.ProjectID]; ok {
				p.UpdatedAt = time.Now()
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
	sessData, err := json.MarshalIndent(m.sessions, "", "  ")
	if err != nil {
		return err
	}
	_ = os.WriteFile(m.storagePath, sessData, 0600)

	projData, err := json.MarshalIndent(m.projects, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.projPath, projData, 0600)
}

func (m *Manager) loadFromDisk() error {
	// Load projects
	if projBytes, err := os.ReadFile(m.projPath); err == nil {
		_ = json.Unmarshal(projBytes, &m.projects)
	}

	// Load sessions
	data, err := os.ReadFile(m.storagePath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &m.sessions); err != nil {
		return err
	}

	for _, s := range m.sessions {
		if s.Type == "" {
			if s.ProjectPath != "" {
				s.Type = SessionTypeProject
			} else {
				s.Type = SessionTypeConversation
			}
		}
		if s.ProjectPath != "" && s.ProjectName == "" {
			s.ProjectName = filepath.Base(s.ProjectPath)
		}

		// Ensure project exists for project session
		if s.Type == SessionTypeProject && s.ProjectPath != "" && s.ProjectID == "" {
			for _, p := range m.projects {
				if p.Path == s.ProjectPath {
					s.ProjectID = p.ID
					break
				}
			}
			if s.ProjectID == "" {
				summary, techStack := InspectProject(s.ProjectPath)
				proj := &Project{
					ID:        fmt.Sprintf("proj_%s", NewUUID()),
					Name:      s.ProjectName,
					Path:      s.ProjectPath,
					Summary:   summary,
					TechStack: techStack,
					CreatedAt: s.CreatedAt,
					UpdatedAt: s.LastActivity,
				}
				m.projects[proj.ID] = proj
				s.ProjectID = proj.ID
			}
		}
	}

	return nil
}
