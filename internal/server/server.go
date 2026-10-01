package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bob/internal/agent"
	"bob/internal/audit"
	"bob/internal/coding"
	"bob/internal/config"
	"bob/internal/sessions"
)

type Server struct {
	cfg            *config.Config
	agent          *agent.Agent
	sessionManager *sessions.Manager
	auditLogger    *audit.Logger
	codingAgent    coding.CodingAgent
	wsManager      *WSManager
	httpServer     *http.Server
	webDir         string
}

func NewServer(
	cfg *config.Config,
	ag *agent.Agent,
	sm *sessions.Manager,
	al *audit.Logger,
	ca coding.CodingAgent,
	webDir string,
) *Server {
	wsm := NewWSManager()
	s := &Server{
		cfg:            cfg,
		agent:          ag,
		sessionManager: sm,
		auditLogger:    al,
		codingAgent:    ca,
		wsManager:      wsm,
		webDir:         webDir,
	}

	// Forward agent events to WebSocket clients
	ag.AddEventListener(func(ev agent.Event) {
		wsm.Broadcast(ev)
	})

	return s
}

// Handler returns the configured HTTP handler with all routes and auth middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public Health Endpoint
	mux.HandleFunc("GET /health", s.handleHealth)

	// API Endpoints
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("POST /api/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("GET /api/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/projects", s.handleCreateOrOpenProject)
	mux.HandleFunc("GET /api/projects/{id}", s.handleGetProject)
	mux.HandleFunc("DELETE /api/projects/{id}", s.handleDeleteProject)
	mux.HandleFunc("POST /api/projects/{id}/sessions", s.handleCreateProjectSession)
	mux.HandleFunc("GET /api/filesystem/browse", s.handleBrowseFilesystem)
	mux.HandleFunc("GET /api/filesystem/recent-projects", s.handleRecentProjects)
	mux.HandleFunc("POST /api/tasks", s.handleCreateTask)
	mux.HandleFunc("GET /api/tasks", s.handleListTasks)
	mux.HandleFunc("GET /api/tasks/{id}", s.handleGetTask)
	mux.HandleFunc("GET /api/tasks/{id}/events", s.handleTaskSSE)
	mux.HandleFunc("POST /api/tasks/{id}/cancel", s.handleCancelTask)
	mux.HandleFunc("POST /api/tasks/{id}/approve", s.handleApproveTask)
	mux.HandleFunc("GET /api/screenshots/{filename}", s.handleGetScreenshot)
	mux.HandleFunc("GET /api/audit", s.handleGetAudit)

	// Coding Subagent Endpoint
	mux.HandleFunc("POST /api/coding/execute", s.handleCodingExecute)

	// WebSocket Endpoint
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Static Assets & UI Handler
	mux.HandleFunc("/", s.handleStaticOrSPA)

	return AuthMiddleware(s.cfg.Server.APIToken, mux)
}

// Start launches the HTTP and WebSocket server.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	handler := s.Handler()

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // Allow long SSE and WS streams
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown stops the server gracefully.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
		"agent":  "Bob",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	sysInfo := config.DetectSystemInfo()
	caAvail, caReason := s.codingAgent.Available()

	status := map[string]any{
		"agent_name": "Bob",
		"status":     "online",
		"system":     sysInfo,
		"user":       s.cfg.User,
		"llm": map[string]any{
			"provider": s.cfg.LLM.Provider,
			"model":    s.cfg.LLM.Model,
			"base_url": s.cfg.LLM.BaseURL,
		},
		"security": map[string]any{
			"require_approval": s.cfg.Security.RequireApprovalForSensitive,
			"allowed_paths":    s.cfg.Filesystem.AllowedPaths,
		},
		"tailscale": map[string]any{
			"enabled": s.cfg.Tailscale.Enabled,
			"ip":      sysInfo.TailscaleIP,
		},
		"coding_agent": map[string]any{
			"name":      s.codingAgent.Name(),
			"available": caAvail,
			"reason":    caReason,
		},
	}

	writeJSON(w, http.StatusOK, status)
}

type CreateSessionRequest struct {
	ID          string               `json:"id,omitempty"`
	Type        sessions.SessionType `json:"type,omitempty"`
	ProjectPath string               `json:"project_path,omitempty"`
	Title       string               `json:"title,omitempty"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var in CreateSessionRequest
	_ = json.NewDecoder(r.Body).Decode(&in)

	if in.Type == sessions.SessionTypeProject || in.ProjectPath != "" {
		if strings.TrimSpace(in.ProjectPath) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_path is required for project sessions"})
			return
		}

		expanded := in.ProjectPath
		if strings.HasPrefix(expanded, "~") {
			home, _ := os.UserHomeDir()
			expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~"))
		}

		absPath, err := filepath.Abs(expanded)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project_path"})
			return
		}
		stat, err := os.Stat(absPath)
		if err != nil || !stat.IsDir() {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("project directory not found: %s", absPath)})
			return
		}
		in.ProjectPath = absPath
		in.Type = sessions.SessionTypeProject
	} else {
		in.Type = sessions.SessionTypeConversation
	}

	sess := s.sessionManager.GetOrCreateWithOptions(in.ID, in.Type, in.ProjectPath, in.Title)
	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.sessionManager.List()
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, ok := s.sessionManager.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}

	sessionTasks := make([]*agent.Task, 0, len(sess.TaskIDs))
	for _, tid := range sess.TaskIDs {
		if t, ok := s.agent.GetTask(tid); ok {
			sessionTasks = append(sessionTasks, t)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session": sess,
		"tasks":   sessionTasks,
	})
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if ok := s.sessionManager.Delete(id); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if ok := s.sessionManager.DeleteProject(id); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projs := s.sessionManager.ListProjects()
	writeJSON(w, http.StatusOK, projs)
}

type CreateProjectRequest struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

func (s *Server) handleCreateOrOpenProject(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}

	trimmed := strings.TrimSpace(req.Path)
	if trimmed == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}

	expanded := trimmed
	if strings.HasPrefix(expanded, "~") {
		home, _ := os.UserHomeDir()
		expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~"))
	}

	absPath, err := filepath.Abs(expanded)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}

	stat, err := os.Stat(absPath)
	if err != nil || !stat.IsDir() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("directory not found: %s", absPath)})
		return
	}

	name := req.Name
	if name == "" {
		name = filepath.Base(absPath)
	}

	proj := s.sessionManager.GetOrCreateProject(absPath, name)

	// Create initial session for this project if none exist
	var initialSession *sessions.Session
	projsWithSess := s.sessionManager.ListProjects()
	for _, p := range projsWithSess {
		if p.ID == proj.ID {
			if len(p.Sessions) > 0 {
				initialSession = p.Sessions[0]
			}
			break
		}
	}
	if initialSession == nil {
		initialSession, _ = s.sessionManager.CreateProjectSession(proj.ID, "General Discussion")
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"project": proj,
		"session": initialSession,
	})
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	proj, ok := s.sessionManager.GetProject(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	writeJSON(w, http.StatusOK, proj)
}

func (s *Server) handleCreateProjectSession(w http.ResponseWriter, r *http.Request) {
	projID := r.PathValue("id")
	var req struct {
		Title string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	sess, err := s.sessionManager.CreateProjectSession(projID, req.Title)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

type CreateTaskRequest struct {
	SessionID string `json:"session_id"`
	Prompt    string `json:"prompt"`
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
		return
	}

	task := s.agent.CreateTask(req.SessionID, req.Prompt)
	s.agent.Run(context.Background(), task)

	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.agent.ListTasks()
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, ok := s.agent.GetTask(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.agent.CancelTask(id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled", "task_id": id})
}

func (s *Server) handleApproveTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Approved bool `json:"approved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}

	if err := s.agent.ApproveTool(id, req.Approved); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "processed", "approved": req.Approved})
}

// Server-Sent Events (SSE) Stream
func (s *Server) handleTaskSSE(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, ok := s.agent.GetTask(id)
	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Emit initial past events
	for _, ev := range task.Events {
		data, _ := json.Marshal(ev)
		fmt.Fprintf(w, "data: %s\n\n", string(data))
	}
	flusher.Flush()

	eventCh := make(chan agent.Event, 32)
	listener := func(ev agent.Event) {
		if ev.TaskID == id {
			select {
			case eventCh <- ev:
			default:
			}
		}
	}

	s.agent.AddEventListener(listener)

	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-eventCh:
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
			if ev.Type == agent.EventTaskCompleted {
				return
			}
		}
	}
}

func (s *Server) handleGetScreenshot(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(r.PathValue("filename"))
	filePath := filepath.Join(s.cfg.Storage.ScreenshotsDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	http.ServeFile(w, r, filePath)
}

func (s *Server) handleGetAudit(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	entries, err := s.auditLogger.ReadRecent(limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleCodingExecute(w http.ResponseWriter, r *http.Request) {
	var req coding.CodingTask
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid coding task request"})
		return
	}

	res, err := s.codingAgent.Execute(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error(), "result": res})
		return
	}

	writeJSON(w, http.StatusOK, res)
}

type BrowseItem struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsProject bool   `json:"is_project"`
}

type BrowseResponse struct {
	CurrentPath string       `json:"current_path"`
	ParentPath  string       `json:"parent_path"`
	Directories []BrowseItem `json:"directories"`
}

func (s *Server) handleBrowseFilesystem(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	if strings.TrimSpace(reqPath) == "" {
		home, _ := os.UserHomeDir()
		reqPath = home
	} else {
		if strings.HasPrefix(reqPath, "~") {
			home, _ := os.UserHomeDir()
			reqPath = filepath.Join(home, strings.TrimPrefix(reqPath, "~"))
		}
	}

	absPath, err := filepath.Abs(reqPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("cannot read directory: %v", err)})
		return
	}

	var dirs []BrowseItem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(absPath, name)
		isProj := checkIsProjectDir(full)
		dirs = append(dirs, BrowseItem{
			Name:      name,
			Path:      full,
			IsProject: isProj,
		})
	}

	parent := filepath.Dir(absPath)
	if parent == absPath {
		parent = ""
	}

	writeJSON(w, http.StatusOK, BrowseResponse{
		CurrentPath: absPath,
		ParentPath:  parent,
		Directories: dirs,
	})
}

type ProjectSummary struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (s *Server) handleRecentProjects(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	candidateDirs := []string{
		filepath.Join(home, "Projects"),
		filepath.Join(home, "Documents"),
		filepath.Join(home, "Desktop"),
		filepath.Join(home, "src"),
		filepath.Join(home, "code"),
		filepath.Join(home, "Work"),
		filepath.Join(home, "Developer"),
	}

	var projects []ProjectSummary
	seen := make(map[string]bool)

	for _, cand := range candidateDirs {
		entries, err := os.ReadDir(cand)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			full := filepath.Join(cand, e.Name())
			if seen[full] {
				continue
			}
			if checkIsProjectDir(full) {
				seen[full] = true
				projects = append(projects, ProjectSummary{
					Name: e.Name(),
					Path: full,
				})
			}
		}
	}

	// Also add projects from existing sessions
	for _, sess := range s.sessionManager.List() {
		if sess.Type == sessions.SessionTypeProject && sess.ProjectPath != "" && !seen[sess.ProjectPath] {
			if info, err := os.Stat(sess.ProjectPath); err == nil && info.IsDir() {
				seen[sess.ProjectPath] = true
				name := sess.ProjectName
				if name == "" {
					name = filepath.Base(sess.ProjectPath)
				}
				projects = append([]ProjectSummary{{Name: name, Path: sess.ProjectPath}}, projects...)
			}
		}
	}

	writeJSON(w, http.StatusOK, projects)
}

func checkIsProjectDir(dir string) bool {
	indicators := []string{
		".git", "package.json", "go.mod", "Cargo.toml", "pyproject.toml",
		"requirements.txt", "Makefile", "pom.xml", "build.gradle", "composer.json",
	}
	for _, ind := range indicators {
		if _, err := os.Stat(filepath.Join(dir, ind)); err == nil {
			return true
		}
	}
	return false
}

func (s *Server) handleStaticOrSPA(w http.ResponseWriter, r *http.Request) {
	if s.webDir == "" {
		s.serveFallbackIndex(w, r)
		return
	}

	path := filepath.Join(s.webDir, filepath.Clean(r.URL.Path))
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}

	indexPath := filepath.Join(s.webDir, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		http.ServeFile(w, r, indexPath)
		return
	}

	s.serveFallbackIndex(w, r)
}

func (s *Server) serveFallbackIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(embeddedFallbackHTML))
}

func writeJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

const embeddedFallbackHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Bob — Local AI Agent</title>
  <style>
    :root { --bg: #0d1117; --panel: #161b22; --border: #30363d; --text: #c9d1d9; --accent: #58a6ff; --green: #238636; --red: #da3633; --yellow: #d29922; }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: var(--bg); color: var(--text); display: flex; flex-direction: column; height: 100vh; }
    header { background: var(--panel); border-bottom: 1px solid var(--border); padding: 12px 20px; display: flex; justify-content: space-between; align-items: center; }
    .logo { font-size: 1.25rem; font-weight: 700; color: #fff; display: flex; align-items: center; gap: 8px; }
    .status-badge { font-size: 0.8rem; padding: 4px 8px; border-radius: 12px; background: rgba(35, 134, 54, 0.2); color: #3fb950; border: 1px solid #238636; }
    .container { display: flex; flex: 1; overflow: hidden; }
    .sidebar { width: 280px; background: var(--panel); border-right: 1px solid var(--border); display: flex; flex-direction: column; padding: 16px; gap: 16px; }
    .main { flex: 1; display: flex; flex-direction: column; }
    .chat-history { flex: 1; overflow-y: auto; padding: 20px; display: flex; flex-direction: column; gap: 16px; }
    .msg { max-width: 80%; padding: 12px 16px; border-radius: 8px; line-height: 1.5; font-size: 0.95rem; }
    .msg.user { align-self: flex-end; background: #1f6feb; color: #fff; }
    .msg.bob { align-self: flex-start; background: #21262d; border: 1px solid var(--border); }
    .tool-box { margin-top: 8px; background: #0d1117; border: 1px solid var(--border); border-radius: 6px; padding: 10px; font-family: ui-monospace, monospace; font-size: 0.85rem; }
    .tool-box summary { cursor: pointer; color: var(--accent); font-weight: 600; }
    .tool-box pre { margin-top: 8px; white-space: pre-wrap; word-break: break-all; color: #8b949e; }
    .input-bar { padding: 16px 20px; background: var(--panel); border-top: 1px solid var(--border); display: flex; gap: 12px; }
    input[type="text"] { flex: 1; background: #0d1117; border: 1px solid var(--border); border-radius: 6px; padding: 10px 14px; color: #fff; font-size: 1rem; outline: none; }
    input[type="text"]:focus { border-color: var(--accent); }
    button { background: var(--green); color: #fff; border: none; border-radius: 6px; padding: 10px 20px; font-weight: 600; cursor: pointer; }
    button:hover { opacity: 0.9; }
    .approval-banner { background: rgba(210, 153, 34, 0.15); border: 1px solid var(--yellow); padding: 12px; border-radius: 6px; margin-top: 10px; }
    .btn-group { display: flex; gap: 8px; margin-top: 8px; }
    .btn-approve { background: var(--green); }
    .btn-reject { background: var(--red); }
  </style>
</head>
<body>
  <header>
    <div class="logo">🤖 BOB <span style="font-weight:400;font-size:0.9rem;color:#8b949e">Local Computer Agent</span></div>
    <div id="status" class="status-badge">● Online</div>
  </header>
  <div class="container">
    <div class="sidebar">
      <div>
        <h4 style="font-size:0.8rem;text-transform:uppercase;color:#8b949e;margin-bottom:8px;">System Info</h4>
        <div id="sysinfo" style="font-size:0.85rem;line-height:1.6;color:#c9d1d9;">Detecting...</div>
      </div>
      <div style="flex:1;">
        <h4 style="font-size:0.8rem;text-transform:uppercase;color:#8b949e;margin-bottom:8px;">Recent Tasks</h4>
        <div id="taskList" style="font-size:0.85rem;display:flex;flex-direction:column;gap:6px;"></div>
      </div>
    </div>
    <div class="main">
      <div id="chat" class="chat-history">
        <div class="msg bob">
          👋 Hi, I am <strong>Bob</strong>. I'm running locally on your Mac. Tell me what you want to do (e.g. "Run tests", "Check git status", "Take screenshot").
        </div>
      </div>
      <div class="input-bar">
        <input id="promptInput" type="text" placeholder="Type a task or question for Bob..." onkeydown="if(event.key==='Enter') sendTask()" />
        <button id="sendBtn" onclick="sendTask()">Send</button>
      </div>
    </div>
  </div>
  <script>
    let ws;
    let currentTaskID = null;

    function initWS() {
      const loc = window.location;
      const wsProto = loc.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = wsProto + '//' + loc.host + '/ws';
      ws = new WebSocket(wsUrl);

      ws.onmessage = (e) => {
        const data = JSON.parse(e.data);
        handleEvent(data);
      };
      ws.onclose = () => {
        document.getElementById('status').innerText = '○ Reconnecting...';
        document.getElementById('status').style.color = '#da3633';
        setTimeout(initWS, 2000);
      };
      ws.onopen = () => {
        document.getElementById('status').innerText = '● Online';
        document.getElementById('status').style.color = '#3fb950';
      };
    }

    async function loadStatus() {
      try {
        const res = await fetch('/api/status');
        const data = await res.json();
        document.getElementById('sysinfo').innerHTML =
          '<strong>Host:</strong> ' + (data.system.macos_version || 'macOS') + ' (' + data.system.architecture + ')<br>' +
          '<strong>CPU:</strong> ' + (data.system.cpu_model || 'Apple Silicon') + '<br>' +
          '<strong>RAM:</strong> ' + (data.system.ram_formatted || 'N/A') + '<br>' +
          '<strong>LLM:</strong> ' + data.llm.model + ' (' + data.llm.provider + ')<br>' +
          '<strong>Tailscale:</strong> ' + (data.tailscale.ip || 'Inactive') + '<br>';
      } catch (err) {
        console.error(err);
      }
    }

    function appendMsg(sender, text, isHtml = false) {
      const chat = document.getElementById('chat');
      const div = document.createElement('div');
      div.className = 'msg ' + sender;
      if (isHtml) div.innerHTML = text; else div.innerText = text;
      chat.appendChild(div);
      chat.scrollTop = chat.scrollHeight;
      return div;
    }

    function handleEvent(ev) {
      if (ev.type === 'tool.started') {
        appendMsg('bob', '<div class="tool-box"><summary>🔧 ' + ev.tool + '</summary><pre>' + (ev.input || '') + '</pre></div>', true);
      } else if (ev.type === 'tool.approval_required') {
        currentTaskID = ev.task_id;
        appendMsg('bob', '<div class="approval-banner"><strong>⚠️ Approval Required:</strong> ' + ev.message + '<br><small>Command: ' + ev.input + '</small><div class="btn-group"><button class="btn-approve" onclick="approve(true)">Approve</button><button class="btn-reject" onclick="approve(false)">Reject</button></div></div>', true);
      } else if (ev.type === 'tool.completed') {
        let content = '<summary>✓ ' + ev.tool + ' (' + ev.duration_ms + 'ms)</summary><pre>' + (ev.output || ev.error || '') + '</pre>';
        if (ev.tool_result && ev.tool_result.data && ev.tool_result.data.url) {
          content += '<br><img src="' + ev.tool_result.data.url + '" style="max-width:100%;border-radius:4px;margin-top:6px;border:1px solid #30363d;">';
        }
        appendMsg('bob', '<div class="tool-box">' + content + '</div>', true);
      } else if (ev.type === 'agent.message') {
        appendMsg('bob', ev.message);
      } else if (ev.type === 'agent.error') {
        appendMsg('bob', '❌ Error: ' + ev.error);
      }
    }

    function sendTask() {
      const input = document.getElementById('promptInput');
      const text = input.value.trim();
      if (!text) return;

      appendMsg('user', text);
      input.value = '';

      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ action: 'create_task', prompt: text }));
      } else {
        fetch('/api/tasks', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ prompt: text })
        });
      }
    }

    function approve(val) {
      if (!currentTaskID) return;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ action: 'approve_tool', task_id: currentTaskID, approved: val }));
      } else {
        fetch('/api/tasks/' + currentTaskID + '/approve', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ approved: val })
        });
      }
      currentTaskID = null;
    }

    window.onload = () => {
      initWS();
      loadStatus();
    };
  </script>
</body>
</html>`
