package sessions

import (
	"os"
	"regexp"
	"testing"
	"time"

	"bob/internal/llm"
)

func TestNewUUID(t *testing.T) {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	id1 := NewUUID()
	id2 := NewUUID()

	if id1 == id2 {
		t.Fatalf("expected unique UUIDs, got duplicates: %s", id1)
	}

	if !uuidRegex.MatchString(id1) {
		t.Errorf("UUID %s does not match RFC 4122 v4 pattern", id1)
	}
	if !uuidRegex.MatchString(id2) {
		t.Errorf("UUID %s does not match RFC 4122 v4 pattern", id2)
	}
}

func TestSessionManager_GetOrCreate_And_Persist(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bob_sessions_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	// Create new session without ID
	sess1 := mgr.GetOrCreate("")
	if sess1.ID == "" {
		t.Fatalf("expected non-empty session ID")
	}
	if sess1.Title != "New Conversation" {
		t.Errorf("expected default title 'New Conversation', got %q", sess1.Title)
	}

	// Retrieve existing session
	sess1Retrieved, ok := mgr.Get(sess1.ID)
	if !ok || sess1Retrieved.ID != sess1.ID {
		t.Errorf("failed to retrieve created session")
	}

	// Append message and check title generation
	mgr.AppendMessage(sess1.ID, llm.Message{
		Role:    llm.RoleUser,
		Content: "What is the capital of France?",
	})

	sessAfterMsg, _ := mgr.Get(sess1.ID)
	if len(sessAfterMsg.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sessAfterMsg.Messages))
	}
	if sessAfterMsg.Title != "What is the capital of France?" {
		t.Errorf("expected title update to 'What is the capital of France?', got %q", sessAfterMsg.Title)
	}

	// Associate task
	mgr.AssociateTask(sess1.ID, "task_123")
	sessAfterTask, _ := mgr.Get(sess1.ID)
	if len(sessAfterTask.TaskIDs) != 1 || sessAfterTask.TaskIDs[0] != "task_123" {
		t.Errorf("failed to associate task with session")
	}

	// Create second session with explicit custom UUID
	customUUID := NewUUID()
	sess2 := mgr.GetOrCreate(customUUID)
	if sess2.ID != customUUID {
		t.Errorf("expected session ID %s, got %s", customUUID, sess2.ID)
	}

	// Check List sorting by last activity
	time.Sleep(10 * time.Millisecond)
	mgr.AppendMessage(sess1.ID, llm.Message{
		Role:    llm.RoleAssistant,
		Content: "Paris.",
	})

	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(list))
	}
	if list[0].ID != sess1.ID {
		t.Errorf("expected sess1 to be first in list due to recent activity, got %s", list[0].ID)
	}

	// Verify persistence: create new manager on same storage path
	mgr2 := NewManager(tmpDir)
	reloadedSess, ok := mgr2.Get(sess1.ID)
	if !ok {
		t.Fatalf("expected reloaded manager to find session %s", sess1.ID)
	}
	if len(reloadedSess.Messages) != 2 {
		t.Errorf("expected 2 reloaded messages, got %d", len(reloadedSess.Messages))
	}
	if reloadedSess.Title != "What is the capital of France?" {
		t.Errorf("expected persisted title to match, got %q", reloadedSess.Title)
	}
}

func TestSessionManager_ProjectAndConversationTypes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bob_project_sessions_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	// Create project session
	projPath := "/tmp/my-sample-project"
	projSess := mgr.Create(SessionTypeProject, projPath, "")
	if projSess.Type != SessionTypeProject {
		t.Errorf("expected session type project, got %s", projSess.Type)
	}
	if projSess.ProjectName != "my-sample-project" {
		t.Errorf("expected project name 'my-sample-project', got %s", projSess.ProjectName)
	}
	if projSess.ProjectPath != projPath {
		t.Errorf("expected project path %s, got %s", projPath, projSess.ProjectPath)
	}
	if projSess.Title != "my-sample-project" {
		t.Errorf("expected title to match project name, got %s", projSess.Title)
	}

	// Create conversation session
	convSess := mgr.Create(SessionTypeConversation, "", "")
	if convSess.Type != SessionTypeConversation {
		t.Errorf("expected session type conversation, got %s", convSess.Type)
	}
	if convSess.Title != "New Conversation" {
		t.Errorf("expected title 'New Conversation', got %s", convSess.Title)
	}

	// Verify persistence
	mgr2 := NewManager(tmpDir)
	reloadedProj, ok := mgr2.Get(projSess.ID)
	if !ok || reloadedProj.Type != SessionTypeProject || reloadedProj.ProjectName != "my-sample-project" {
		t.Errorf("failed to reload project session correctly: %+v", reloadedProj)
	}
	reloadedConv, ok := mgr2.Get(convSess.ID)
	if !ok || reloadedConv.Type != SessionTypeConversation {
		t.Errorf("failed to reload conversation session correctly: %+v", reloadedConv)
	}
}

func TestSessionManager_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bob_delete_sessions_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	// Create project and sessions
	proj := mgr.GetOrCreateProject("/tmp/test-proj-del", "test-proj-del")
	sess1, err := mgr.CreateProjectSession(proj.ID, "Chat 1")
	if err != nil {
		t.Fatalf("failed to create project session: %v", err)
	}
	sess2, err := mgr.CreateProjectSession(proj.ID, "Chat 2")
	if err != nil {
		t.Fatalf("failed to create project session: %v", err)
	}
	conv := mgr.Create(SessionTypeConversation, "", "General Chat")

	// Delete individual session
	if !mgr.Delete(sess1.ID) {
		t.Errorf("expected Delete to return true for existing session")
	}
	if _, ok := mgr.Get(sess1.ID); ok {
		t.Errorf("expected sess1 to be deleted from manager")
	}

	// Deleting non-existent should return false
	if mgr.Delete("non_existent_id") {
		t.Errorf("expected Delete to return false for non-existent session")
	}

	// Delete project
	if !mgr.DeleteProject(proj.ID) {
		t.Errorf("expected DeleteProject to return true for existing project")
	}
	if _, ok := mgr.GetProject(proj.ID); ok {
		t.Errorf("expected project to be deleted")
	}
	if _, ok := mgr.Get(sess2.ID); ok {
		t.Errorf("expected project session sess2 to be deleted when project is deleted")
	}

	// conv should still exist
	if _, ok := mgr.Get(conv.ID); !ok {
		t.Errorf("expected conv session to remain")
	}

	// Verify persistence after deletion
	mgr2 := NewManager(tmpDir)
	if _, ok := mgr2.Get(sess1.ID); ok {
		t.Errorf("expected sess1 to not exist in reloaded manager")
	}
	if _, ok := mgr2.Get(sess2.ID); ok {
		t.Errorf("expected sess2 to not exist in reloaded manager")
	}
	if _, ok := mgr2.Get(conv.ID); !ok {
		t.Errorf("expected conv to exist in reloaded manager")
	}
}

