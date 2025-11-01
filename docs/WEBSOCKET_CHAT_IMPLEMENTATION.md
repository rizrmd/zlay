# WebSocket Chat Implementation with OpenAI Integration

## Table of Contents
1. [Current State Analysis](#current-state-analysis)
2. [Architecture Overview](#architecture-overview)
3. [Backend Implementation](#backend-implementation)
4. [Frontend Implementation](#frontend-implementation)
5. [Database Schema](#database-schema)
6. [WebSocket Message Protocol](#websocket-message-protocol)
7. [OpenAI Integration](#openai-integration)
8. [Tool Execution Framework](#tool-execution-framework)
9. [Implementation Phases](#implementation-phases)
10. [Security & Performance](#security--performance)

## Current State Analysis

### What Currently Exists
- **Frontend**: Complete chat UI with message input, display, and history
- **Backend**: Basic CRUD APIs (auth, projects, datasources, admin)
- **Authentication**: Cookie-based session management
- **Multi-tenant**: Client and domain-based isolation
- **Database**: PostgreSQL with connection pooling
- **Client AI Config**: `ai_api_key`, `ai_api_url`, `ai_api_model` fields

### What's Missing
- **No real chat transmission**: Frontend uses simulated responses
- **No chat endpoints**: Backend has no WebSocket or chat API endpoints
- **No LLM integration**: No OpenAI or other provider connections
- **No tool system**: No framework for external tool execution
- **No real-time communication**: No WebSocket or SSE implementation

## Architecture Overview

### System Components
```
Frontend (Vue.js) ←→ WebSocket ←→ Go Backend ←→ OpenAI API
                                    ↓
                              Tool Execution Framework
                                    ↓
                              Datasources (PostgreSQL, APIs, etc.)
```

### Key Principles
1. **Real-time Communication**: WebSocket for bidirectional messaging
2. **Project Isolation**: Each chat session scoped to a project
3. **Tool Integration**: Seamless tool calling via OpenAI function calling
4. **Permission-based Access**: Tools respect project and user permissions
5. **Scalable Architecture**: Support for multiple LLM providers

## Backend Implementation

### Directory Structure
```
backend/
├── internal/
│   ├── websocket/
│   │   ├── hub.go          # WebSocket connection hub
│   │   ├── manager.go      # Connection lifecycle management
│   │   └── handler.go      # WebSocket message routing
│   ├── chat/
│   │   ├── service.go       # Chat business logic
│   │   ├── conversation.go  # Conversation management
│   │   └── message.go      # Message processing
│   ├── llm/
│   │   ├── client.go       # LLM provider interface
│   │   ├── openai.go       # OpenAI client implementation
│   │   └── streaming.go     # Streaming response handling
│   ├── tools/
│   │   ├── registry.go      # Tool registration system
│   │   ├── executor.go      # Tool execution engine
│   │   ├── database.go     # Database query tools
│   │   ├── http.go         # HTTP API tools
│   │   └── filesystem.go   # File system tools
│   └── models/
│       ├── conversation.go  # Conversation data models
│       ├── message.go      # Message data models
│       └── tool.go         # Tool execution models
├── cmd/
│   └── websocket/         # WebSocket server setup
└── migrations/
    └── 002_chat_tables.sql # Chat-related database migrations
```

### WebSocket Hub Implementation

#### Connection Management
```go
// internal/websocket/hub.go
type Hub struct {
    connections    map[*Connection]bool
    projects      map[string]map[*Connection]bool // Project-based rooms
    broadcast     chan []byte
    register      chan *Connection
    unregister    chan *Connection
    projectJoin   chan *ProjectJoin
    projectLeave  chan *ProjectLeave
}

type Connection struct {
    ws        *websocket.Conn
    send      chan []byte
    userID    string
    clientID  string
    projectID string
}

type ProjectJoin struct {
    connection *Connection
    projectID  string
}
```

#### Message Handler
```go
// internal/websocket/handler.go
func (h *Hub) HandleMessage(conn *Connection, msgType int, payload []byte) {
    var message WebSocketMessage
    if err := json.Unmarshal(payload, &message); err != nil {
        // Send error response
        return
    }

    switch message.Type {
    case "user_message":
        h.handleUserMessage(conn, message)
    case "join_project":
        h.handleProjectJoin(conn, message)
    case "leave_project":
        h.handleProjectLeave(conn, message)
    }
}
```

### Chat Service Implementation

#### Chat Processing Flow
```go
// internal/chat/service.go
func (s *ChatService) ProcessMessage(ctx context.Context, req *ChatRequest) error {
    // 1. Save user message
    userMsg := &Message{
        ConversationID: req.ConversationID,
        Role:          "user",
        Content:        req.Content,
        UserID:         req.UserID,
        ProjectID:      req.ProjectID,
    }
    if err := s.repo.SaveMessage(ctx, userMsg); err != nil {
        return err
    }

    // 2. Get conversation history
    history, err := s.repo.GetConversationHistory(ctx, req.ConversationID)
    if err != nil {
        return err
    }

    // 3. Call OpenAI with context
    openaiReq := &openai.ChatCompletionRequest{
        Model:       req.Project.AIModel,
        Messages:     historyToOpenAIMessages(history),
        Functions:   s.toolRegistry.GetAvailableTools(req.ProjectID),
        Stream:      true,
    }

    // 4. Stream response back to client
    return s.llmClient.StreamChat(ctx, openaiReq, func(chunk *openai.ChatCompletionStreamResponse) {
        s.hub.BroadcastToProject(req.ProjectID, WebSocketMessage{
            Type: "assistant_response",
            Data: AssistantResponse{
                Content:    chunk.Choices[0].Delta.Content,
                MessageID:  chunk.ID,
                Timestamp:  time.Now(),
            },
        },
    })
}
```

### Tool Execution Framework

#### Tool Registry
```go
// internal/tools/registry.go
type ToolRegistry struct {
    tools map[string]Tool
    mutex sync.RWMutex
}

type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]ToolParameter
    Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error)
    ValidateAccess(userID, projectID string) bool
}

// Example database tool
type DatabaseQueryTool struct {
    db *pgxpool.Pool
}

func (t *DatabaseQueryTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
    query := params["query"].(string)
    rows, err := t.db.Query(ctx, query)
    // Execute query and return results
    return &ToolResult{Data: rows}, nil
}
```

## Frontend Implementation

### Directory Structure
```
frontend/src/
├── services/
│   ├── api.ts           # Existing API client
│   └── websocket.ts     # WebSocket client service
├── stores/
│   ├── chat.ts          # Chat state management (Pinia)
│   └── websocket.ts     # WebSocket connection state
├── composables/
│   ├── useChat.ts       # Chat functionality composable
│   └── useWebSocket.ts  # WebSocket connection composable
└── views/
    └── ChatView.vue      # Enhanced chat interface
```

### WebSocket Service
```typescript
// src/services/websocket.ts
export class WebSocketService {
    private ws: WebSocket | null = null
    private reconnectAttempts = 0
    private maxReconnectAttempts = 5
    private messageHandlers = new Map<string, Function>()
    private projectID: string | null = null

    connect(projectID: string, token: string): Promise<void> {
        return new Promise((resolve, reject) => {
            const wsUrl = `${this.getWebSocketURL()}/ws/chat?token=${token}&project=${projectID}`
            this.ws = new WebSocket(wsUrl)

            this.ws.onopen = () => {
                console.log('WebSocket connected')
                this.projectID = projectID
                this.reconnectAttempts = 0
                resolve()
            }

            this.ws.onmessage = (event) => {
                const message = JSON.parse(event.data)
                this.handleMessage(message)
            }

            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error)
                reject(error)
            }

            this.ws.onclose = () => {
                console.log('WebSocket disconnected')
                this.handleReconnect()
            }
        })
    }

    sendMessage(type: string, data: any): void {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({
                type,
                data,
                timestamp: Date.now(),
            }))
        }
    }

    private handleMessage(message: WebSocketMessage): void {
        const handler = this.messageHandlers.get(message.type)
        if (handler) {
            handler(message.data)
        }
    }
}
```

### Chat Store (Pinia)
```typescript
// src/stores/chat.ts
export const useChatStore = defineStore('chat', () => {
    const conversations = ref<Map<string, Conversation>>(new Map())
    const currentConversation = ref<string | null>(null)
    const messages = ref<Message[]>([])
    const isLoading = ref(false)

    const sendMessage = async (content: string) => {
        if (!currentConversation.value) return

        const userMessage: Message = {
            id: generateId(),
            role: 'user',
            content,
            timestamp: new Date(),
        }

        messages.value.push(userMessage)
        isLoading.value = true

        // Send via WebSocket
        webSocketService.sendMessage('user_message', {
            conversation_id: currentConversation.value,
            content,
        })
    }

    const createConversation = async (projectID: string, title: string): Promise<string> => {
        const conversation = {
            id: generateId(),
            project_id: projectID,
            title,
            created_at: new Date(),
        }

        conversations.value.set(conversation.id, conversation)
        currentConversation.value = conversation.id
        messages.value = []

        return conversation.id
    }

    return {
        conversations,
        currentConversation,
        messages,
        isLoading,
        sendMessage,
        createConversation,
    }
})
```

### Enhanced ChatView.vue
```vue
<template>
  <div class="flex h-screen">
    <!-- Sidebar with conversations -->
    <Sidebar />
    
    <!-- Main chat area -->
    <div class="flex-1 flex flex-col">
      <!-- Messages -->
      <div ref="messagesContainer" class="flex-1 overflow-y-auto">
        <MessageComponent 
          v-for="message in messages" 
          :key="message.id"
          :message="message"
        />
        
        <!-- Streaming indicator -->
        <div v-if="isStreaming" class="flex items-center space-x-2">
          <div class="animate-pulse">AI is thinking...</div>
        </div>
      </div>

      <!-- Input -->
      <div class="border-t p-4">
        <ChatInput 
          @send="handleSendMessage"
          :disabled="isLoading"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useChatStore } from '@/stores/chat'
import { useWebSocket } from '@/composables/useWebSocket'

const { messages, sendMessage, isLoading } = useChatStore()
const { isConnected, isStreaming } = useWebSocket()

const handleSendMessage = (content: string) => {
  sendMessage(content)
}

onMounted(async () => {
  // Connect to WebSocket
  const projectID = route.params.id as string
  await webSocketService.connect(projectID, getAuthToken())
  
  // Load conversation history
  await loadConversationHistory()
})
</script>
```

## Database Schema

### Chat Tables
```sql
-- Conversations table
CREATE TABLE IF NOT EXISTS conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Messages table
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    tool_calls JSONB DEFAULT '[]',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Tool executions table
CREATE TABLE IF NOT EXISTS tool_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    tool_name VARCHAR(255) NOT NULL,
    tool_parameters JSONB NOT NULL,
    tool_result JSONB,
    execution_status VARCHAR(20) DEFAULT 'pending' CHECK (execution_status IN ('pending', 'executing', 'completed', 'failed')),
    execution_time_ms INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_conversations_project_id ON conversations(project_id);
CREATE INDEX IF NOT EXISTS idx_conversations_user_id ON conversations(user_id);
CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_tool_executions_message_id ON tool_executions(message_id);
```

## WebSocket Message Protocol

### Message Types
```typescript
// Base message structure
interface WebSocketMessage {
    type: string
    data: any
    timestamp: number
    id?: string
}

// User message
interface UserMessageData {
    conversation_id: string
    content: string
    project_id: string
}

// Assistant response (streaming)
interface AssistantResponseData {
    conversation_id: string
    content: string
    message_id: string
    timestamp: Date
    done: boolean // Indicates streaming completion
}

// Tool call request
interface ToolCallData {
    conversation_id: string
    tool_calls: ToolCall[]
}

// Tool execution result
interface ToolResultData {
    tool_call_id: string
    tool_name: string
    result: any
    status: 'executing' | 'completed' | 'failed'
    error?: string
}
```

### Message Flow Examples

#### User sends message
```json
{
    "type": "user_message",
    "data": {
        "conversation_id": "conv-123",
        "content": "Show me all users from the database",
        "project_id": "proj-456"
    },
    "timestamp": 1699123456789
}
```

#### Assistant streaming response
```json
{
    "type": "assistant_response",
    "data": {
        "conversation_id": "conv-123",
        "content": "I'll help you query",
        "message_id": "msg-789",
        "done": false
    },
    "timestamp": 1699123456789
}
```

#### Tool execution
```json
{
    "type": "tool_call",
    "data": {
        "conversation_id": "conv-123",
        "tool_calls": [
            {
                "id": "call-123",
                "function": {
                    "name": "database_query",
                    "arguments": "{\"query\": \"SELECT * FROM users LIMIT 10\"}"
                }
            }
        ]
    }
}
```

## OpenAI Integration

### Client Setup
```go
// internal/llm/openai.go
package llm

import (
    "github.com/openai/openai-go"
    "context"
)

type OpenAIClient struct {
    client   *openai.Client
    model    string
    apiKey   string
    baseURL  string
}

func NewOpenAIClient(apiKey, baseURL, model string) *OpenAIClient {
    config := openai.DefaultConfig(apiKey)
    if baseURL != "" {
        config.BaseURL = baseURL
    }
    
    return &OpenAIClient{
        client: openai.NewClientWithConfig(config),
        model:  model,
        apiKey: apiKey,
        baseURL: baseURL,
    }
}
```

### Streaming Chat Implementation
```go
func (c *OpenAIClient) StreamChat(ctx context.Context, req *ChatCompletionRequest, callback func(*StreamingChunk)) error {
    // Convert internal request to OpenAI request
    openaiReq := &openai.ChatCompletionRequest{
        Model:       c.model,
        Messages:     req.Messages,
        Functions:   req.Functions,
        Stream:      true,
    }

    // Start streaming
    stream, err := c.client.Chat.Completions.Stream(ctx, openaiReq)
    if err != nil {
        return err
    }

    // Process stream chunks
    for stream.Next() {
        chunk := stream.Current()
        callback(&StreamingChunk{
            Content: chunk.Choices[0].Delta.Content,
            Done:    chunk.Choices[0].FinishReason != "",
        })
    }

    return stream.Err()
}
```

### Function/Tool Calling
```go
func (c *OpenAIClient) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
    // Convert tools to OpenAI functions
    functions := make([]openai.FunctionDefinition, len(tools))
    for i, tool := range tools {
        functions[i] = openai.FunctionDefinition{
            Name:        tool.Name(),
            Description: tool.Description(),
            Parameters:   tool.Parameters(),
        }
    }

    req := &openai.ChatCompletionRequest{
        Model:       c.model,
        Messages:     messagesToOpenAI(messages),
        Functions:   functions,
        Stream:      false, // Get complete response for function calls
    }

    resp, err := c.client.Chat.Completions(ctx, req)
    if err != nil {
        return nil, err
    }

    return &ChatResponse{
        Content:   resp.Choices[0].Message.Content,
        FunctionCalls: resp.Choices[0].Message.FunctionCalls,
    }, nil
}
```

## Tool Execution Framework

### Built-in Tools

#### Database Query Tool
```go
type DatabaseQueryTool struct {
    db     *pgxpool.Pool
    project *Project
}

func (t *DatabaseQueryTool) Name() string { return "database_query" }
func (t *DatabaseQueryTool) Description() string { return "Execute SQL queries on project datasources" }

func (t *DatabaseQueryTool) Parameters() map[string]ToolParameter {
    return map[string]ToolParameter{
        "datasource_id": {
            Type:        "string",
            Description: "ID of the datasource to query",
            Required:     true,
        },
        "query": {
            Type:        "string",
            Description: "SQL query to execute",
            Required:     true,
        },
    }
}

func (t *DatabaseQueryTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
    datasourceID := params["datasource_id"].(string)
    query := params["query"].(string)
    
    // Get datasource configuration
    datasource, err := t.getDatasource(ctx, datasourceID)
    if err != nil {
        return nil, err
    }
    
    // Execute query using zlay-db
    zdb, err := t.createDatabaseConnection(datasource)
    if err != nil {
        return nil, err
    }
    
    result, err := zdb.Query(ctx, query)
    if err != nil {
        return &ToolResult{
            Status: "failed",
            Error:  err.Error(),
        }, nil
    }
    
    return &ToolResult{
        Status: "completed",
        Data:   result,
    }, nil
}
```

#### HTTP API Tool
```go
type HTTPTool struct {
    client *http.Client
}

func (t *HTTPTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
    method := params["method"].(string)
    url := params["url"].(string)
    headers := params["headers"].(map[string]string)
    body := params["body"]
    
    req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
    if err != nil {
        return nil, err
    }
    
    for key, value := range headers {
        req.Header.Set(key, value)
    }
    
    resp, err := t.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    
    return &ToolResult{
        Status: "completed",
        Data: map[string]interface{}{
            "status_code": resp.StatusCode,
            "headers":    resp.Header,
            "body":       string(respBody),
        },
    }, nil
}
```

### Tool Execution Pipeline
```go
func (e *ToolExecutor) ExecuteToolCall(ctx context.Context, call *ToolCall) (*ToolResult, error) {
    // 1. Validate tool exists
    tool, exists := e.registry.GetTool(call.Function.Name)
    if !exists {
        return nil, fmt.Errorf("tool '%s' not found", call.Function.Name)
    }
    
    // 2. Validate user permissions
    if !tool.ValidateAccess(call.UserID, call.ProjectID) {
        return nil, fmt.Errorf("access denied for tool '%s'", call.Function.Name)
    }
    
    // 3. Parse parameters
    params, err := json.Unmarshal(call.Function.Arguments.Raw, &map[string]interface{}{})
    if err != nil {
        return nil, fmt.Errorf("invalid parameters: %w", err)
    }
    
    // 4. Execute with timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    result, err := tool.Execute(ctx, *params)
    if err != nil {
        return &ToolResult{
            Status: "failed",
            Error:  err.Error(),
        }, nil
    }
    
    return result, nil
}
```

## Implementation Phases

### Phase 1: Basic WebSocket Infrastructure (Week 1)
1. **Backend WebSocket Hub**
   - Implement connection management
   - Add project-based rooms
   - Basic message routing

2. **Frontend WebSocket Client**
   - Connection service
   - Basic message handling
   - Reconnection logic

3. **Database Schema**
   - Create conversation/message tables
   - Add migration scripts

### Phase 2: OpenAI Integration (Week 2)
1. **OpenAI Client**
   - Initialize client with project config
   - Basic chat completion
   - Error handling

2. **Chat Service**
   - Message persistence
   - Conversation management
   - OpenAI integration

3. **Frontend Chat UI**
   - Replace mock responses
   - Real-time message display
   - Conversation history

### Phase 3: Tool System (Week 3)
1. **Tool Registry**
   - Tool interface definition
   - Basic tool implementations
   - Permission validation

2. **Tool Executor**
   - Execution pipeline
   - Error handling
   - Result formatting

3. **Built-in Tools**
   - Database query tool
   - HTTP API tool
   - File system tool

### Phase 4: Advanced Features (Week 4)
1. **OpenAI Function Calling**
   - Tool calling integration
   - Streaming with function results
   - Parameter validation

2. **Enhanced UI**
   - Tool execution visualization
   - Real-time status updates
   - Error handling

3. **Performance Optimization**
   - Connection pooling
   - Message queuing
   - Rate limiting

### Phase 5: Production Readiness (Week 5)
1. **Security**
   - Input validation
   - Access control
   - Audit logging

2. **Monitoring**
   - Metrics collection
   - Error tracking
   - Performance monitoring

3. **Testing**
   - Unit tests
   - Integration tests
   - Load testing

## Security & Performance

### Security Considerations

#### Authentication & Authorization
```go
// WebSocket authentication middleware
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Validate session token
        token := r.URL.Query().Get("token")
        user, err := validateSessionToken(token)
        if err != nil {
            http.Error(w, "Unauthorized", 401)
            return
        }
        
        // Add user context
        ctx := context.WithValue(r.Context(), "user", user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

#### Tool Access Control
```go
func (t *DatabaseQueryTool) ValidateAccess(userID, projectID string) bool {
    // Check if user has access to project
    var hasAccess bool
    err := db.QueryRow(ctx,
        `SELECT EXISTS(
            SELECT 1 FROM project_members 
            WHERE user_id = $1 AND project_id = $2
        )`, userID, projectID).Scan(&hasAccess)
    
    return err == nil && hasAccess
}
```

#### Input Validation
```go
func validateMessage(msg *WebSocketMessage) error {
    switch msg.Type {
    case "user_message":
        return validateUserMessage(msg.Data)
    case "tool_call":
        return validateToolCall(msg.Data)
    default:
        return fmt.Errorf("unknown message type: %s", msg.Type)
    }
}
```

### Performance Optimization

#### Connection Management
```go
// Connection pool for database tools
type ToolDBPool struct {
    pools map[string]*pgxpool.Pool
    mutex sync.RWMutex
}

func (p *ToolDBPool) GetConnection(datasourceID string) (*pgxpool.Pool, error) {
    p.mutex.RLock()
    pool, exists := p.pools[datasourceID]
    p.mutex.RUnlock()
    
    if exists {
        return pool, nil
    }
    
    // Create new connection pool
    p.mutex.Lock()
    defer p.mutex.Unlock()
    
    pool, err := createPool(datasourceID)
    if err != nil {
        return nil, err
    }
    
    p.pools[datasourceID] = pool
    return pool, nil
}
```

#### Message Queuing
```go
// Async message processing
type MessageQueue struct {
    queue chan *WebSocketMessage
    workers int
}

func (q *MessageQueue) Start() {
    for i := 0; i < q.workers; i++ {
        go q.worker()
    }
}

func (q *MessageQueue) worker() {
    for msg := range q.queue {
        // Process message asynchronously
        q.processMessage(msg)
    }
}
```

#### Caching Strategy
```go
// Tool result caching
type ToolCache struct {
    cache map[string]*CacheEntry
    ttl   time.Duration
    mutex sync.RWMutex
}

func (c *ToolCache) Get(key string) (*ToolResult, bool) {
    c.mutex.RLock()
    entry, exists := c.cache[key]
    c.mutex.RUnlock()
    
    if !exists || time.Since(entry.Timestamp) > c.ttl {
        return nil, false
    }
    
    return entry.Result, true
}
```

### Monitoring & Metrics

#### Performance Metrics
```go
type Metrics struct {
    MessageCount    prometheus.Counter
    ResponseTime    prometheus.Histogram
    ActiveConnections prometheus.Gauge
    ToolExecutions  prometheus.Counter
    Errors          prometheus.Counter
}
```

#### Error Tracking
```go
func (s *ChatService) ProcessMessageWithRecovery(ctx context.Context, req *ChatRequest) {
    defer func() {
        if r := recover(); r != nil {
            log.Error("Panic in ProcessMessage", "panic", r)
            s.metrics.Errors.Inc()
            // Send error response to client
            s.hub.SendError(req.ConnectionID, "Internal server error")
        }
    }()
    
    s.ProcessMessage(ctx, req)
}
```

## Conclusion

This implementation provides a complete real-time chat system with:
- **WebSocket-based communication** for low-latency messaging
- **OpenAI integration** with streaming responses and function calling
- **Extensible tool system** for database queries, API calls, and file operations
- **Project-based isolation** respecting existing multi-tenant architecture
- **Security and performance** considerations for production deployment

The phased implementation approach allows for incremental development and testing, ensuring each component is robust before moving to the next phase.
