package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"bob/internal/agent"
	"bob/internal/audit"
	"bob/internal/coding"
	"bob/internal/config"
	"bob/internal/llm"
	"bob/internal/security"
	"bob/internal/server"
	"bob/internal/sessions"
	"bob/internal/tools/registry"
)

func setupTestServer(t *testing.T, token string) (*server.Server, *config.Config, *agent.Agent, string) {
	tempDir := t.TempDir()
	webDir := filepath.Join(tempDir, "web")
	_ = os.MkdirAll(webDir, 0755)
	_ = os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<html><body>Bob SPA</body></html>"), 0644)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:     "127.0.0.1",
			Port:     8787,
			APIToken: token,
		},
		LLM: config.LLMConfig{
			Provider: "mock",
			Model:    "mock-model",
		},
		Agent: config.AgentConfig{
			MaxSteps:       5,
			MaxToolCalls:   5,
			TimeoutSeconds: 5,
		},
		Security: config.SecurityConfig{
			RequireApprovalForSensitive: true,
		},
		Filesystem: config.FilesystemConfig{
			AllowedPaths: []string{tempDir},
		},
		Storage: config.StorageConfig{
			DataDir:        tempDir,
			AuditLogPath:   tempDir + "/audit.jsonl",
			ScreenshotsDir: tempDir + "/screenshots",
		},
	}

	mockLLM := llm.NewMockLLM(llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "Mock response from API test.",
		},
		FinishReason: "stop",
	})

	reg := registry.NewRegistry()
	policy := security.NewCommandPolicy([]string{"echo"}, []string{"rm"}, []string{"rm -rf /"})
	redactor := security.NewRedactor(token)
	auditLog, _ := audit.NewLogger(cfg.Storage.AuditLogPath, redactor)
	sessMgr := sessions.NewManager(tempDir)
	ca := coding.NewAntiGravityCodingAgent(false)

	ag := agent.NewAgent(mockLLM, reg, policy, auditLog, sessMgr, agent.AgentConfig{
		MaxSteps:       cfg.Agent.MaxSteps,
		MaxToolCalls:   cfg.Agent.MaxToolCalls,
		TimeoutSeconds: cfg.Agent.TimeoutSeconds,
	})

	srv := server.NewServer(cfg, ag, sessMgr, auditLog, ca, webDir)
	return srv, cfg, ag, webDir
}

func TestServer_AuthMiddleware(t *testing.T) {
	token := "secure_secret_token_123"
	srv, _, _, _ := setupTestServer(t, token)
	handler := srv.Handler()

	// 1. Health check is always public without token
	healthReq := httptest.NewRequest("GET", "/health", nil)
	healthRec := httptest.NewRecorder()
	handler.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Errorf("expected public /health to return 200, got %d", healthRec.Code)
	}

	// 2. Unauthorized request to protected endpoint
	apiReq := httptest.NewRequest("GET", "/api/status", nil)
	apiRec := httptest.NewRecorder()
	handler.ServeHTTP(apiRec, apiReq)
	if apiRec.Code != http.StatusUnauthorized {
		t.Errorf("expected unauthorized request to return 401, got %d", apiRec.Code)
	}

	// 3. Authorized request with Bearer token
	authReq := httptest.NewRequest("GET", "/api/status", nil)
	authReq.Header.Set("Authorization", "Bearer "+token)
	authRec := httptest.NewRecorder()
	handler.ServeHTTP(authRec, authReq)
	if authRec.Code != http.StatusOK {
		t.Errorf("expected authorized request to return 200, got %d", authRec.Code)
	}
}

func TestServer_TaskCreation(t *testing.T) {
	_, cfg, ag, _ := setupTestServer(t, "")

	task := ag.CreateTask("", "Hello Bob API")
	ag.Run(context.Background(), task)

	time.Sleep(50 * time.Millisecond)

	fetchedTask, ok := ag.GetTask(task.ID)
	if !ok {
		t.Fatalf("expected task %s to exist", task.ID)
	}

	if fetchedTask.Prompt != "Hello Bob API" {
		t.Errorf("got prompt %q, want %q", fetchedTask.Prompt, "Hello Bob API")
	}

	_ = cfg
}

func TestServer_SessionEndpoints_And_ChatRoutes(t *testing.T) {
	srv, _, _, _ := setupTestServer(t, "")
	handler := srv.Handler()

	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	// 1. POST /api/sessions (creates session with unique UUID)
	createReq := httptest.NewRequest("POST", "/api/sessions", bytes.NewBuffer([]byte("{}")))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d", createRec.Code)
	}

	var createdSess sessions.Session
	if err := json.NewDecoder(createRec.Body).Decode(&createdSess); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !uuidRegex.MatchString(createdSess.ID) {
		t.Errorf("expected session ID to be a UUID v4, got: %s", createdSess.ID)
	}

	// 2. GET /api/sessions/{id}
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/sessions/%s", createdSess.ID), nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", getRec.Code)
	}

	var sessionDetail struct {
		Session sessions.Session `json:"session"`
		Tasks   []agent.Task     `json:"tasks"`
	}
	if err := json.NewDecoder(getRec.Body).Decode(&sessionDetail); err != nil {
		t.Fatalf("failed to decode session detail: %v", err)
	}
	if sessionDetail.Session.ID != createdSess.ID {
		t.Errorf("expected session ID %s, got %s", createdSess.ID, sessionDetail.Session.ID)
	}

	// 3. GET /c/{id} (SPA Route should serve index.html)
	spaReq := httptest.NewRequest("GET", fmt.Sprintf("/c/%s", createdSess.ID), nil)
	spaRec := httptest.NewRecorder()
	handler.ServeHTTP(spaRec, spaReq)

	if spaRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for SPA route, got %d", spaRec.Code)
	}

	body, _ := io.ReadAll(spaRec.Body)
	if !bytes.Contains(body, []byte("Bob SPA")) {
		t.Errorf("expected SPA route /c/{id} to return index.html, got %s", string(body))
	}

	// 4. DELETE /api/sessions/{id}
	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/sessions/%s", createdSess.ID), nil)
	delRec := httptest.NewRecorder()
	handler.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusOK {
		t.Errorf("expected DELETE /api/sessions/{id} to return 200, got %d", delRec.Code)
	}

	// Subsequent GET should return 404
	getAfterDel := httptest.NewRequest("GET", fmt.Sprintf("/api/sessions/%s", createdSess.ID), nil)
	getAfterDelRec := httptest.NewRecorder()
	handler.ServeHTTP(getAfterDelRec, getAfterDel)
	if getAfterDelRec.Code != http.StatusNotFound {
		t.Errorf("expected GET after DELETE to return 404, got %d", getAfterDelRec.Code)
	}
}
