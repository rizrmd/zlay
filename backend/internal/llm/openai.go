package llm

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// OpenAIClient implements LLMClient for OpenAI
type OpenAIClient struct {
	client    *openai.Client
	model     string
	apiKey    string
	baseURL   string
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey, baseURL, model string) *OpenAIClient {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	
	client := openai.NewClient(opts...)
	return &OpenAIClient{
		client: &client,
		model:  model,
		apiKey: apiKey,
		baseURL: baseURL,
	}
}

// StreamChat implements LLMClient interface with streaming
func (c *OpenAIClient) StreamChat(ctx context.Context, req *LLMRequest, callback func(*StreamingChunk) error) error {
	// For now, use non-streaming API and simulate streaming
	// TODO: Implement proper streaming with official OpenAI Go SDK when streaming API is available
	
	resp, err := c.Chat(ctx, req)
	if err != nil {
		return err
	}

	// Simulate streaming by sending chunks of the response
	content := resp.Content
	if content == "" {
		content = " "
	}

	// Split content into words for simulated streaming
	words := strings.Fields(content)
	for i, word := range words {
		chunk := &StreamingChunk{
			Content: word + " ",
			Done:    i == len(words)-1,
		}
		
		if err := callback(chunk); err != nil {
			return err
		}
	}

	// Send final chunk with tool calls if any
	if resp.ToolCalls != nil {
		finalChunk := &StreamingChunk{
			Content:   "",
			ToolCalls: resp.ToolCalls,
			Done:      true,
		}
		return callback(finalChunk)
	}

	// Send completion chunk
	completionChunk := &StreamingChunk{
		Content: "",
		Done:    true,
	}

	return callback(completionChunk)
}

// Chat implements LLMClient interface for non-streaming
func (c *OpenAIClient) Chat(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	// Set default model if not specified
	model := req.Model
	if model == "" {
		model = c.model
	}

	// Create OpenAI request
	chatService := (*c.client).Chat.Completions.New
	openaiReq := openai.ChatCompletionNewParams{
		Model:       model,
		Messages:    req.Messages,
		MaxTokens:   openai.Int(int64(req.MaxTokens)),
		Temperature: openai.Float(float64(req.Temperature)),
		Tools:       req.Tools,
	}

	// Make request
	resp, err := chatService(ctx, openaiReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in OpenAI response")
	}

	choice := resp.Choices[0]
	
	// Convert tool calls
	// Convert tool calls from OpenAI format to our format
	var toolCalls interface{}
	if len(choice.Message.ToolCalls) > 0 {
		toolCalls = choice.Message.ToolCalls
	}

	// Build response
	response := &LLMResponse{
		Content:   choice.Message.Content,
		ToolCalls: toolCalls,
		Usage:     &resp.Usage,
		Model:     model,
	}

	return response, nil
}

// SetModel updates the model for this client
func (c *OpenAIClient) SetModel(model string) error {
	c.model = model
	log.Printf("OpenAI client model updated to: %s", model)
	return nil
}

// GetModel returns the current model
func (c *OpenAIClient) GetModel() string {
	return c.model
}

// ValidateConnection tests if the OpenAI connection works
func (c *OpenAIClient) ValidateConnection(ctx context.Context) error {
	// Make a simple API call to test connection
	testReq := &LLMRequest{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello"),
		},
		MaxTokens: 10,
	}

	resp, err := c.Chat(ctx, testReq)
	if err != nil {
		return fmt.Errorf("OpenAI connection test failed: %w", err)
	}

	if resp == nil || resp.Content == "" {
		return fmt.Errorf("OpenAI connection test: invalid response")
	}

	log.Printf("OpenAI connection validated successfully")
	return nil
}

// EstimateTokens estimates the number of tokens for a text
func (c *OpenAIClient) EstimateTokens(text string) (int, error) {
	// This is a rough estimation - OpenAI doesn't provide a simple tokenizer
	// GPT-3.5 and GPT-4 use ~1 token per 4 characters
	estimated := len(text) / 4
	if estimated < 1 {
		estimated = 1
	}
	return estimated, nil
}

// GetContextWindow returns the maximum context window for the current model
func (c *OpenAIClient) GetContextWindow() int {
	// Return context window sizes for common models
	switch c.model {
	case "gpt-3.5-turbo", "gpt-3.5-turbo-16k":
		return 16385
	case "gpt-4", "gpt-4-32k":
		return 8192
	case "gpt-4-turbo", "gpt-4-1106-preview":
		return 128000
	default:
		// Default to 4096 for unknown models
		return 4096
	}
}

// GetMaxTokens returns the recommended max tokens for the current model
func (c *OpenAIClient) GetMaxTokens() int {
	contextWindow := c.GetContextWindow()
	// Reserve some tokens for the response
	return contextWindow / 2
}

// Helper functions

// formatMessage converts our message format to OpenAI format
func (c *OpenAIClient) formatMessages(messages []openai.ChatCompletionMessageParamUnion) []openai.ChatCompletionMessageParamUnion {
	var formatted []openai.ChatCompletionMessageParamUnion
	
	for _, msg := range messages {
		// Deep copy the message to avoid modifying original
		formatted = append(formatted, msg)
	}
	
	return formatted
}

// isRetryableError checks if an error can be retried
func (c *OpenAIClient) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	
	// Check for retryable OpenAI errors
	retryableErrors := []string{
		"timeout",
		"connection reset",
		"temporary failure",
		"rate limit",
		"server error",
	}
	
	for _, retryableErr := range retryableErrors {
		if strings.Contains(errStr, retryableErr) {
			return true
		}
	}
	
	return false
}

// retryWithBackoff implements exponential backoff retry
func (c *OpenAIClient) retryWithBackoff(ctx context.Context, operation func() error, maxRetries int) error {
	var lastErr error
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := operation(); err == nil {
			return nil
		} else {
			lastErr = err
			
			// Don't retry if error is not retryable
			if !c.isRetryableError(err) {
				return err
			}
			
			// Calculate backoff delay (exponential with jitter)
			backoffMs := (1 << uint(attempt)) * 100 // 100ms, 200ms, 400ms, 800ms...
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(backoffMs) * time.Millisecond):
				log.Printf("OpenAI operation failed (attempt %d/%d), retrying in %dms: %v", attempt+1, maxRetries, backoffMs, err)
			}
		}
	}
	
	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, lastErr)
}
