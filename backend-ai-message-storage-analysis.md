# Backend AI Message Storage Analysis

## Question: What happens when AI messages are not done yet?

## Short Answer

**AI messages are NOT saved to database until streaming is complete.** They exist only in memory during streaming.

## Detailed Analysis

### **1. Message Lifecycle During Streaming**

#### **During Streaming (chunk.Done = false)**
```go
// In callback function for each chunk
callback := func(chunk *llm.StreamingChunk) error {
    // Accumulate content in memory ONLY
    if chunk.Content != "" {
        assistantMsg.Content += chunk.Content  // ❌ Not saved to DB yet
        assistantMsg.CreatedAt = time.Now()
    }

    // Send chunk to WebSocket immediately
    response := &msglib.WebSocketMessage{
        Type: "assistant_response",
        Data: gin.H{
            "content": chunk.Content,
            "done": chunk.Done,  // false during streaming
        },
    }
    s.hub.BroadcastToProject(req.ProjectID, &response)  // ✅ Broadcast only
}
```

#### **After Streaming Completes (chunk.Done = true)**
```go
// When final chunk arrives
if chunk.Done {
    // Send final chunk with done=true
    // ... (same as above)
}

// AFTER STREAMING ENDS:
// 1. Process tool calls if any
if len(assistantMsg.ToolCalls) > 0 {
    s.processToolCalls(ctx, req, assistantMsg)
}

// 2. Save complete message to database
if err := s.saveMessage(ctx, assistantMsg); err != nil {
    log.Printf("Failed to save assistant message: %v", err)
}

// 3. Send completion message
completionResponse := WebSocketMessage{
    Type: "assistant_response",
    Data: gin.H{
        "done": true,  // ✅ Final completion message
    },
}
s.hub.BroadcastToProject(req.ProjectID, &completionResponse)
```

### **2. Message Storage Timeline**

```
User sends message
↓
Backend creates assistant message placeholder in memory
assistantMsg := NewMessage(conversationID, "assistant", "", ...)
↓
Streaming starts (multiple chunks)
Chunk 1: "Hello" → assistantMsg.Content = "Hello" → Broadcast (DB: ❌)
Chunk 2: " there" → assistantMsg.Content = "Hello there" → Broadcast (DB: ❌)
Chunk 3: "!" → assistantMsg.Content = "Hello there!" → Broadcast (DB: ❌)
...
↓
Streaming completes (chunk.Done = true)
↓
Save complete message to database: "Hello there!" → ✅ DB Save
↓
Send completion message with done=true
```

### **3. Database Persistence Pattern**

#### **What IS Saved During Streaming**
```go
// ❌ NOTHING during streaming
// Only in-memory: assistantMsg.Content += chunk.Content

// ✅ Only AFTER streaming completes
func (s *chatService) saveMessage(ctx context.Context, msg *Message) error {
    query := `
        INSERT INTO messages (id, conversation_id, role, content, metadata, tool_calls, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `
    // Complete message saved as single row
}
```

#### **Database State During Streaming**
```sql
-- Before streaming starts
INSERT INTO messages VALUES ('user-msg-1', 'conv-1', 'user', 'Hello', ...)

-- During streaming (no entries)
-- ❌ No assistant message in database yet

-- After streaming completes
INSERT INTO messages VALUES ('assistant-msg-1', 'conv-1', 'assistant', 'Hello there!', ...)

-- Final database state
+----------------+----------------+------------+---------------+
| id             | conversation_id | role       | content       |
+----------------+----------------+------------+---------------+
| user-msg-1     | conv-1          | user       | Hello         |
| assistant-msg-1 | conv-1          | assistant  | Hello there!  |
+----------------+----------------+------------+---------------+
```

## **4. Implications for Direct URL Loading**

### **Problem for Streaming Continuation**
When user opens direct URL during active streaming:

```
1. Frontend loads conversations from database
   ↓
2. Database contains: user message + NO assistant message
   ↓
3. Frontend shows only user message
   ↓
4. Backend is still streaming but database has no assistant message
   ↓
5. No way to know about ongoing streaming from database
```

### **Current Backend Limitations**
```go
// ❌ Backend doesn't track streaming state
func (h *Handler) handleGetConversationStatus(conn *Connection, message *WebSocketMessage) {
    // Currently defaults to false - no actual tracking
    isProcessing := false
}

// ❌ No persistence of streaming state
// When server restarts: all streaming state lost
```

## **5. Memory vs Database During Streaming**

### **Memory State (Streaming)**
```go
assistantMsg := NewMessage(...)  // In memory only
assistantMsg.Content = ""      // Empty initially

// During streaming callback
assistantMsg.Content += chunk.Content  // Accumulates in memory
// Result after 3 chunks: "Hello there!"
```

### **Database State (After Completion)**
```sql
-- Single complete message
INSERT INTO messages VALUES ('msg-123', 'conv-1', 'assistant', 'Hello there!', ...)
```

## **6. WebSocket Broadcasting Pattern**

### **During Streaming**
```go
// Multiple messages, all with done=false
WebSocketMessage{Type: "assistant_response", Data: gin.H{"content": "H", "done": false}}
WebSocketMessage{Type: "assistant_response", Data: gin.H{"content": "e", "done": false}}
WebSocketMessage{Type: "assistant_response", Data: gin.H{"content": "l", "done": false}}
WebSocketMessage{Type: "assistant_response", Data: gin.H{"content": "l", "done": false}}
WebSocketMessage{Type: "assistant_response", Data: gin.H{"content": "o", "done": false}}
WebSocketMessage{Type: "assistant_response", Data: gin.H{"content": " ", "done": false}}
...
```

### **After Completion**
```go
// Final message with done=true
WebSocketMessage{Type: "assistant_response", Data: gin.H{"content": "", "done": true}}
// Plus immediate database save of complete message
```

## **7. Edge Cases and Failure Scenarios**

### **Server Crash During Streaming**
```go
// Before crash: assistantMsg.Content = "Hello there!" (memory)
// Database: No assistant message
// After restart: "Hello there!" is lost forever ❌
```

### **WebSocket Disconnection During Streaming**
```go
// Backend continues streaming in memory
// Frontend misses remaining chunks
// When reconnected: no way to resume ❌
```

### **Multiple Clients Streaming Same Conversation**
```go
// Client A: Receives streaming chunks
// Client B: Receives same chunks (broadcasted)
// Both see partial content in real-time
// Both see final save to database
```

## **8. Current Frontend Workarounds**

### **Frontend Detection (Current Implementation)**
```typescript
// Frontend guesses incomplete messages
const isIncomplete = 
  !lastMessage.content ||                    // Empty content
  lastMessage.content.endsWith('```') ||     // Unclosed code block
  lastMessage.content.length < 20           // Very short response
```

### **Why Detection is Needed**
- Backend doesn't provide streaming state
- Database has no assistant message during streaming
- Frontend must infer from partial content

## **9. Potential Backend Improvements**

### **Streaming State Tracking**
```go
// Add streaming state to chat service
type ChatService struct {
    streamingConversations map[string]bool  // Track active streams
}

// Update in callback
callback := func(chunk *llm.StreamingChunk) error {
    if !streamStarted && chunk.Content != "" {
        chatService.streamingConversations[req.ConversationID] = true
    }
    if chunk.Done {
        delete(chatService.streamingConversations, req.ConversationID)
    }
}
```

### **Partial Message Saving**
```go
// Option: Save incomplete messages with special flag
if chunk.Content != "" && !streamStarted {
    // Save placeholder message with streaming flag
    saveStreamingMessage(ctx, assistantMsg, isStreaming: true)
}
```

## **Summary**

| Phase | Message State | Database | Memory | WebSocket |
|--------|---------------|-----------|---------|------------|
| **Start** | Empty placeholder | ❌ No | ✅ Broadcast empty |
| **Streaming** | Content accumulates | ❌ No | ✅ Broadcast chunks |
| **Complete** | Full content | ✅ Save | ✅ Broadcast done=true |

**Key Insight**: Assistant messages exist **only in memory** until streaming completes, then they're saved as a single complete database entry.

This explains why direct URLs during streaming need frontend detection - the backend simply doesn't track or persist streaming state.