package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func setupTestServer(t *testing.T, token string) (*server.Server, *config.Config, *agent.Agent) {
	tempDir := t.TempDir()

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

	srv := server.NewServer(cfg, ag, sessMgr, auditLog, ca, "")
	return srv, cfg, ag
}

func TestServer_AuthMiddleware(t *testing.T) {
	token := "secure_secret_token_123"
	_, _, _ = setupTestServer(t, token)

	handler := server.AuthMiddleware(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

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
	srv, cfg, ag := setupTestServer(t, "")

	// Create test server
	go func() {
		_ = srv.Start()
	}()
	time.Sleep(50 * time.Millisecond)
	defer func() { _ = srv.Shutdown(context.Background()) }()

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
