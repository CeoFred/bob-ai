package llm

import (
	"context"
	"sync"
)

// MockLLM provides deterministic scripted responses for testing without a live LLM.
type MockLLM struct {
	mu        sync.Mutex
	responses []ChatResponse
	index     int
	Calls     []ChatRequest
}

func NewMockLLM(responses ...ChatResponse) *MockLLM {
	return &MockLLM{
		responses: responses,
	}
}

func (m *MockLLM) Name() string {
	return "mock"
}

func (m *MockLLM) AddResponse(resp ChatResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, resp)
}

func (m *MockLLM) Chat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, request)

	if len(m.responses) == 0 {
		return ChatResponse{
			Message: Message{
				Role:    RoleAssistant,
				Content: "Default mock response.",
			},
			FinishReason: "stop",
		}, nil
	}

	if m.index >= len(m.responses) {
		// Repeat last response or return fallback
		return m.responses[len(m.responses)-1], nil
	}

	resp := m.responses[m.index]
	m.index++
	return resp, nil
}

func (m *MockLLM) Stream(ctx context.Context, request ChatRequest) (<-chan StreamEvent, error) {
	resp, err := m.Chat(ctx, request)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamEvent, 2)
	go func() {
		defer close(ch)
		ch <- StreamEvent{
			Delta: resp.Message.Content,
			Done:  false,
		}
		ch <- StreamEvent{
			FinishReason: resp.FinishReason,
			Done:         true,
		}
	}()
	return ch, nil
}
