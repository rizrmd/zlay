package llm

import (
	"context"

	"github.com/openai/openai-go"
)

// LLMRequest represents a request to the LLM
type LLMRequest struct {
	Messages []openai.ChatCompletionMessageParamUnion `json:"messages"`
	Tools    []openai.ChatCompletionToolParam `json:"tools,omitempty"`
	Model     string                         `json:"model,omitempty"`
	MaxTokens int                            `json:"max_tokens,omitempty"`
	Temperature float32                       `json:"temperature,omitempty"`
}

// StreamingChunk represents a chunk from streaming LLM response
type StreamingChunk struct {
	Content   string `json:"content"`
	ToolCalls interface{} `json:"tool_calls,omitempty"`
	Done      bool    `json:"done"`
	TokensUsed int     `json:"tokens_used,omitempty"`
}

// LLMClient defines the interface for LLM providers
type LLMClient interface {
	// StreamChat sends a chat completion request and streams the response
	StreamChat(ctx context.Context, req *LLMRequest, callback func(*StreamingChunk) error) error
	
	// Chat sends a chat completion request and returns the complete response
	Chat(ctx context.Context, req *LLMRequest) (*LLMResponse, error)
	
	// SetModel updates the model for this client
	SetModel(model string) error
	
	// GetModel returns the current model
	GetModel() string
}

// LLMResponse represents a complete LLM response
type LLMResponse struct {
	Content    string        `json:"content"`
	ToolCalls  interface{}   `json:"tool_calls,omitempty"`
	Usage      interface{}   `json:"usage,omitempty"`
	Model      string        `json:"model"`
	TokensUsed int           `json:"tokens_used"`
}
