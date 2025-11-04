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

// StreamChat implements LLMClient interface with real streaming
func (c *OpenAIClient) StreamChat(ctx context.Context, req *LLMRequest, callback func(*StreamingChunk) error) error {
	// Set default model if not specified
	model := req.Model
	if model == "" {
		model = c.model
	}

	log.Printf("🚀 StreamChat CALLED:")
	log.Printf("   • Model: %s", model)
	log.Printf("   • Messages Count: %d", len(req.Messages))
	log.Printf("   • Max Tokens: %d", req.MaxTokens)
	log.Printf("   • Temperature: %f", req.Temperature)
	log.Printf("   • Tools Count: %d", len(req.Tools))
	log.Printf("   • Base URL: %s", c.baseURL)

	// Log all messages for debugging
	for i, msg := range req.Messages {
		log.Printf("   • Message %d: Role=%s, Content=%.100s", i+1, msg.GetRole(), msg.GetContent())
	}

	// Create OpenAI streaming request using the correct API
	log.Printf("📡 Creating OpenAI streaming request...")
	stream := (*c.client).Chat.Completions.NewStreaming(ctx, 
		openai.ChatCompletionNewParams{
			Model:       model,
			Messages:    req.Messages,
			MaxTokens:   openai.Int(int64(req.MaxTokens)),
			Temperature: openai.Float(float64(req.Temperature)),
			Tools:       req.Tools,
		},
	)

	log.Printf("📡 OpenAI streaming request created, waiting for first chunk...")
	chunkCount := 0
	totalContent := ""

	// Process streaming response
	log.Printf("📡 STARTING OPENAI STREAMING PROCESSING...")
	for stream.Next() {
		chunk := stream.Current()
		chunkCount++
		
		if len(chunk.Choices) == 0 {
			log.Printf("⚠️ Chunk #%d: No choices available", chunkCount)
			continue
		}

		choice := chunk.Choices[0]
		content := choice.Delta.Content
		totalContent += content
		
		// 🔥 DETAILED LOGGING: Log every chunk for debugging
		log.Printf("📦 OPENAI CHUNK #%d:", chunkCount)
		log.Printf("   • Content: \"%s\"", content)
		log.Printf("   • Content Length: %d", len(content))
		log.Printf("   • Finish Reason: %s", choice.FinishReason)
		log.Printf("   • Total Content Length: %d", len(totalContent))
		log.Printf("   • Has Tool Calls: %t", len(choice.Delta.ToolCalls) > 0)
		
		// Log first chunk and every 10th chunk
		if chunkCount == 1 {
			log.Printf("📥 First chunk received from OpenAI: content='%s', finish_reason='%s'", content, choice.FinishReason)
		} else if chunkCount%10 == 0 {
			log.Printf("📦 Chunk #%d received: content='%s', total_length=%d", chunkCount, content, len(totalContent))
		}
		
		// Create streaming chunk
		streamingChunk := &StreamingChunk{
			Content:   content,
			Done:      choice.FinishReason != "",
			TokensUsed: 0, // Will be calculated from final usage
		}

		// Handle tool calls in streaming
		if len(choice.Delta.ToolCalls) > 0 {
			log.Printf("🔧 Tool calls received in chunk: %+v", choice.Delta.ToolCalls)
			streamingChunk.ToolCalls = choice.Delta.ToolCalls
		}

		// 🔥 DETAILED LOGGING: Log chunk being sent to callback
		log.Printf("📤 SENDING CHUNK TO CALLBACK:")
		log.Printf("   • Content: \"%s\"", streamingChunk.Content)
		log.Printf("   • Content Length: %d", len(streamingChunk.Content))
		log.Printf("   • Done: %t", streamingChunk.Done)
		log.Printf("   • Tokens Used: %d", streamingChunk.TokensUsed)

		// Send chunk to callback
		if err := callback(streamingChunk); err != nil {
			log.Printf("❌ ERROR SENDING CHUNK TO CALLBACK: %v", err)
			return err
		}
		log.Printf("✅ CHUNK SENT TO CALLBACK SUCCESSFULLY")

		// If this is the final chunk, include usage information
		if streamingChunk.Done && chunk.Usage.TotalTokens > 0 {
			log.Printf("✅ Final chunk received! Total chunks: %d, total_content_length: %d, tokens_used: %d", 
				chunkCount, len(totalContent), chunk.Usage.TotalTokens)
				
			finalChunk := &StreamingChunk{
				Content:    "",
				Done:       true,
				TokensUsed: int(chunk.Usage.TotalTokens),
			}
			if err := callback(finalChunk); err != nil {
				log.Printf("❌ Error sending final chunk to callback: %v", err)
				return err
			}
		}
	}

	// Check for streaming errors
	if err := stream.Err(); err != nil {
		log.Printf("❌ OPENAI STREAMING ERROR:")
		log.Printf("   • Total Chunks Processed: %d", chunkCount)
		log.Printf("   • Total Content Length: %d", len(totalContent))
		log.Printf("   • Error: %v", err)
		return fmt.Errorf("OpenAI streaming error: %w", err)
	}

	log.Printf("🏁 OPENAI STREAMING COMPLETED SUCCESSFULLY:")
	log.Printf("   • Total Chunks: %d", chunkCount)
	log.Printf("   • Final Content Length: %d", len(totalContent))
	log.Printf("   • Final Content: \"%s\"", totalContent)
	log.Printf("   • Stream Finished Without Errors")
	return nil
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
	
	// Calculate tokens used
	tokensUsed := 0
	if resp.Usage.TotalTokens > 0 {
		tokensUsed = int(resp.Usage.TotalTokens)
	}

	// Convert tool calls from OpenAI format to our format
	var toolCalls interface{}
	if len(choice.Message.ToolCalls) > 0 {
		toolCalls = choice.Message.ToolCalls
	}

	// Build response
	response := &LLMResponse{
		Content:    choice.Message.Content,
		ToolCalls:  toolCalls,
		Usage:      resp.Usage,
		Model:      model,
		TokensUsed: tokensUsed,
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
	// Test with a simple non-streaming request to validate basic connectivity
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
