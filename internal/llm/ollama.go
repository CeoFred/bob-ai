package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaLLM implements the LLM interface for local Ollama instances.
type OllamaLLM struct {
	baseURL    string
	httpClient *http.Client
	model      string
}

// NewOllamaLLM creates a new Ollama client.
func NewOllamaLLM(baseURL string, defaultModel string, timeout time.Duration) *OllamaLLM {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	return &OllamaLLM{
		baseURL: baseURL,
		model:   defaultModel,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (o *OllamaLLM) Name() string {
	return "ollama"
}

// OllamaChatPayload represents the JSON payload expected by Ollama /api/chat.
type ollamaChatPayload struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
	Stream   bool             `json:"stream"`
	Options  map[string]any   `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role      string            `json:"role"`
	Content   string            `json:"content"`
	ToolCalls []ollamaToolCall  `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaFunctionCall `json:"function"`
}

type ollamaFunctionCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ollamaChatResponse struct {
	Model      string        `json:"model"`
	CreatedAt  string        `json:"created_at"`
	Message    ollamaMessage `json:"message"`
	Done       bool          `json:"done"`
	DoneReason string        `json:"done_reason"`
	Error      string        `json:"error,omitempty"`
}

// Chat sends a non-streaming chat completion request to Ollama.
func (o *OllamaLLM) Chat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	model := request.Model
	if model == "" {
		model = o.model
	}
	if model == "" {
		return ChatResponse{}, ErrInvalidModel
	}

	payload := ollamaChatPayload{
		Model:    model,
		Messages: toOllamaMessages(request.Messages),
		Tools:    request.Tools,
		Stream:   false,
		Options: map[string]any{
			"temperature": request.Temperature,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("failed to encode request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/api/chat", o.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("%w: %v", ErrLLMUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ChatResponse{}, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return ChatResponse{}, fmt.Errorf("failed to parse ollama response: %w", err)
	}

	if ollamaResp.Error != "" {
		return ChatResponse{}, fmt.Errorf("ollama error: %s", ollamaResp.Error)
	}

	// Map to standard ChatResponse
	msg := Message{
		Role:    Role(ollamaResp.Message.Role),
		Content: ollamaResp.Message.Content,
	}

	for i, tc := range ollamaResp.Message.ToolCalls {
		argsBytes, _ := json.Marshal(tc.Function.Arguments)
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{
			ID:   fmt.Sprintf("call_%d", i+1),
			Type: "function",
			Function: FunctionCall{
				Name:      tc.Function.Name,
				Arguments: argsBytes,
			},
		})
	}

	return ChatResponse{
		Message:      msg,
		FinishReason: ollamaResp.DoneReason,
		Model:        ollamaResp.Model,
	}, nil
}

// Stream sends a streaming chat request to Ollama and yields stream events.
func (o *OllamaLLM) Stream(ctx context.Context, request ChatRequest) (<-chan StreamEvent, error) {
	model := request.Model
	if model == "" {
		model = o.model
	}

	payload := ollamaChatPayload{
		Model:    model,
		Messages: toOllamaMessages(request.Messages),
		Tools:    request.Tools,
		Stream:   true,
		Options: map[string]any{
			"temperature": request.Temperature,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode stream request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/api/chat", o.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLLMUnavailable, err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	eventChan := make(chan StreamEvent, 16)

	go func() {
		defer resp.Body.Close()
		defer close(eventChan)

		reader := bufio.NewReader(resp.Body)
		for {
			select {
			case <-ctx.Done():
				eventChan <- StreamEvent{Error: ctx.Err(), Done: true}
				return
			default:
			}

			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					eventChan <- StreamEvent{Error: err, Done: true}
				}
				return
			}

			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			var chunk ollamaChatResponse
			if err := json.Unmarshal(line, &chunk); err != nil {
				continue
			}

			if chunk.Error != "" {
				eventChan <- StreamEvent{Error: fmt.Errorf("%s", chunk.Error), Done: true}
				return
			}

			var tc *ToolCall
			if len(chunk.Message.ToolCalls) > 0 {
				first := chunk.Message.ToolCalls[0]
				argsBytes, _ := json.Marshal(first.Function.Arguments)
				tc = &ToolCall{
					ID:   "call_stream_1",
					Type: "function",
					Function: FunctionCall{
						Name:      first.Function.Name,
						Arguments: argsBytes,
					},
				}
			}

			eventChan <- StreamEvent{
				Delta:        chunk.Message.Content,
				ToolCall:     tc,
				FinishReason: chunk.DoneReason,
				Done:         chunk.Done,
			}

			if chunk.Done {
				return
			}
		}
	}()

	return eventChan, nil
}

func toOllamaMessages(msgs []Message) []ollamaMessage {
	res := make([]ollamaMessage, 0, len(msgs))
	for _, m := range msgs {
		om := ollamaMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
		for _, tc := range m.ToolCalls {
			var args map[string]any
			_ = json.Unmarshal(tc.Function.Arguments, &args)
			om.ToolCalls = append(om.ToolCalls, ollamaToolCall{
				Function: ollamaFunctionCall{
					Name:      tc.Function.Name,
					Arguments: args,
				},
			})
		}
		res = append(res, om)
	}
	return res
}
