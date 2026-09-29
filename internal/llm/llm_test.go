package llm_test

import (
	"context"
	"testing"

	"bob/internal/llm"
)

func TestMockLLM_ChatAndStream(t *testing.T) {
	mockResp := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "Streamed test chunk",
		},
		FinishReason: "stop",
		Model:        "mock-model",
	}

	mock := llm.NewMockLLM(mockResp)
	if mock.Name() != "mock" {
		t.Errorf("got name %s, want mock", mock.Name())
	}

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hello"},
		},
	}

	resp, err := mock.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if resp.Message.Content != "Streamed test chunk" {
		t.Errorf("got %q, want %q", resp.Message.Content, "Streamed test chunk")
	}

	// Test Stream
	ch, err := mock.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("stream failed: %v", err)
	}

	var chunks []string
	for ev := range ch {
		if ev.Delta != "" {
			chunks = append(chunks, ev.Delta)
		}
	}

	if len(chunks) == 0 {
		t.Errorf("expected stream events, got 0")
	}
}
