# Store Refactoring - Project vs Conversation Separation

## 🔥 Problem Solved
Previous `chat.ts` store mixed concerns:
- **Project Management** (WebSocket connection, project state)
- **Conversation Management** (chat messages, conversation data)
- **Chat Functionality** (messaging, streaming, tool execution)

## 🏗️ New Architecture

### **1. useProjectStore** (`/stores/project.ts`)
**Responsibilities:**
- ✅ WebSocket connection management
- ✅ Project state tracking
- ✅ Connection status monitoring
- ✅ Project joining/leaving
- ✅ Health checks & pings

**State:**
```typescript
{
  currentProjectId: string | null
  currentProject: Project | null
  projects: Map<string, Project>
  isConnected: boolean
  connectionStatus: string
  isConnecting: boolean
  isLoadingProjects: boolean
}
```

**Actions:**
- `initWebSocket(projectId)` - Initialize connection
- `disconnectWebSocket()` - Clean disconnect
- `joinProject(projectId)` - Join project room
- `setupWebSocketHandlers()` - Project-specific handlers
- `setCurrentProject(projectId)` - Update active project

---

### **2. useConversationStore** (`/stores/conversation.ts`)
**Responsibilities:**
- ✅ Conversation CRUD operations
- ✅ Message storage & retrieval
- ✅ Conversation history loading
- ✅ Conversation selection & switching
- ✅ Processing state tracking

**State:**
```typescript
{
  conversations: Map<string, Conversation>
  currentConversationId: string | null
  conversationMessages: Map<string, Message[]>
  processingConversations: Set<string>
  isLoading: boolean
  isLoadingHistory: boolean
  messages: Message[]  // UI legacy compatibility
}
```

**Actions:**
- `loadConversations()` - Fetch conversation list
- `loadConversation(id)` - Load specific conversation
- `selectConversation(id)` - Switch active conversation
- `createConversation(title, message)` - New conversation
- `addOrUpdateMessage(conversationId, message)` - Stream message handling
- `addToProcessing(conversationId)` - Track streaming state

---

### **3. useChatStore** (`/stores/chat.ts`)
**Responsibilities:**
- ✅ WebSocket message routing & handling
- ✅ Real-time streaming logic
- ✅ Tool execution status management
- ✅ Chat UI integration
- ✅ URL navigation coordination

**State (Computed from other stores):**
```typescript
{
  // Delegated from stores
  conversations: computed(() => conversationStore.conversations)
  isConnected: computed(() => projectStore.isConnected)
  messages: computed(() => conversationStore.messages)
  
  // Chat-specific state
  streamingState: Map<string, any>
  toolStatuses: Map<string, string>
  isSendingMessage: boolean
  isCreatingConversation: boolean
}
```

**Actions:**
- `initChat(projectId)` - Initialize entire chat system
- `sendMessage(content)` - Send user message
- `setupMessageHandlers()` - Route WebSocket messages
- `handleBaseChatPath(projectId)` - URL redirection logic
- `updateToolCallStatus()` - Tool status management

---

## 🔄 Data Flow

### **Initialization**
```
ChatLayout.vue
    ↓
chatStore.initChat(projectId)
    ↓
1. projectStore.initWebSocket(projectId)
2. chatStore.setupMessageHandlers()
3. conversationStore.loadConversations()
```

### **Message Handling**
```
WebSocket Message → chatStore → appropriate specialized store
assistant_response → conversationStore.addOrUpdateMessage()
conversation_created → conversationStore + URL navigation
project_joined → projectStore
```

### **UI Updates**
```
ChatLayout.vue
    ↓ (computed)
chatStore.conversations → conversationStore.conversations
chatStore.isConnected → projectStore.isConnected
chatStore.messages → conversationStore.messages
```

---

## 🎯 Benefits

### **1. Single Responsibility**
- Each store has clear, focused purpose
- Easier to debug and maintain
- Better code organization

### **2. Scalability**
- Easy to add project-specific features
- Conversation management independent of chat UI
- WebSocket handling centralized

### **3. Testing**
- Each store can be unit tested independently
- Mock dependencies easily
- Clear interfaces between stores

### **4. Type Safety**
- Stronger TypeScript typing
- Clear data flow paths
- Reduced coupling

---

## 🛠️ Migration Guide

### **Before (Mixed Store)**
```typescript
// All functionality in one store
const chatStore = useChatStore()
await chatStore.initWebSocket(projectId)
await chatStore.loadConversations()
await chatStore.sendMessage(content)
```

### **After (Separated Stores)**
```typescript
// Specialized stores
const chatStore = useChatStore()
const projectStore = useProjectStore()
const conversationStore = useConversationStore()

// Coordinated initialization
await chatStore.initChat(projectId)

// Actions go to appropriate stores
await chatStore.sendMessage(content)  // Routes through chat logic
await conversationStore.loadConversations()  // Direct conversation management
await projectStore.joinProject(projectId)  // Direct project management
```

---

## 🔧 Component Updates

### **ChatLayout.vue**
- Imports: `useChatStore`, `useProjectStore`
- Initialization: `chatStore.initChat(projectId)`
- URL Handling: `chatStore.handleBaseChatPath()`

### **Other Components**
- No changes needed - computed properties handle delegation
- All existing APIs maintained through computed delegation

---

## 🚀 Next Steps

### **Potential Enhancements**
1. **useUserStore** - For user profile and preferences
2. **useNotificationStore** - For system notifications
3. **useSettingsStore** - For application settings
4. **useCacheStore** - For offline data management

### **Performance Optimizations**
1. **Store Memoization** - Cache computed values
2. **Lazy Loading** - Load conversations on-demand
3. **Memory Management** - Cleanup inactive conversations

This separation provides a solid foundation for scaling the application while maintaining clean, maintainable code.