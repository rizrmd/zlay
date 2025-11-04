export interface WebSocketMessage {
  type: string
  data: any
  timestamp?: number
  id?: string
  tokens_used?: number
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  metadata?: Record<string, any>
  tool_calls?: ToolCall[]
  created_at: string
  user_id?: string
  project_id?: string
  conversation_id?: string
}

export interface ToolCall {
  id: string
  type: string
  function: {
    name: string
    arguments: any
  }
  status?: string
  result?: any
  error?: string
}

export interface Conversation {
  id: string
  project_id: string
  user_id: string
  title: string
  status: string // processing, completed, interrupted
  created_at: string
  updated_at: string
}

export interface ConversationDetails {
  conversation: Conversation
  messages: ChatMessage[]
  tool_status?: Record<string, string>
}

class WebSocketService {
  private ws: WebSocket | null = null
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000
  private messageHandlers = new Map<string, Function>()
  private projectID: string | null = null
  private connected: boolean = false
  private connectionPromise: Promise<void> | null = null

  // Token usage tracking
  private tokensUsed = 0
  private tokensLimit = 1000000

  async connect(projectID: string): Promise<void> {
    console.log(`Attempting to connect WebSocket for project: ${projectID}`)

    // If already connecting, wait for existing connection
    if (this.connectionPromise) {
      console.log('WebSocket connection already in progress, waiting...')
      return this.connectionPromise
    }

    this.connectionPromise = new Promise<void>((resolve, reject) => {
      try {
        // Determine WebSocket URL
        const wsUrl = this.getWebSocketURL(projectID)
        console.log(`Creating WebSocket connection to: ${wsUrl}`)

        // Create WebSocket with compression enabled
        this.ws = new WebSocket(wsUrl)

        this.ws.onopen = () => {
          console.log('🔗 WebSocket connection opened:', {
            readyState: this.ws?.readyState,
            protocol: this.ws?.protocol,
            extensions: this.ws?.extensions,
            projectID: this.projectID,
            timestamp: new Date().toISOString(),
          })

          this.projectID = projectID
          this.connected = true
          this.reconnectAttempts = 0
          this.reconnectDelay = 1000

          // Set up connection-level handlers
          this.setupConnectionHandlers()

          // 🔄 NEW: Notify connection establishment
          console.log('📤 Sending connection_established message')
          this.sendMessage('connection_established', {
            timestamp: Date.now(),
          })

          resolve()
        }

        this.ws.onmessage = (event) => {
          try {
            const message = JSON.parse(event.data) as WebSocketMessage
            
            // 🔥 DEBUG: Log ALL received messages for debugging
            console.log('📥 WEBSOCKET MESSAGE RECEIVED:', {
              type: message.type,
              hasData: !!message.data,
              timestamp: message.timestamp,
              id: message.id,
              tokens_used: message.tokens_used,
              rawData: message
            })
            
            // Special logging for assistant responses
            if (message.type === 'assistant_response') {
              const content = message.data.content || ''
              console.log('🤖 ASSISTANT RESPONSE CHUNK:', {
                content: `"${content}"`,
                contentLength: content.length,
                done: message.data.done,
                conversation_id: message.data.conversation_id,
                message_id: message.data.message_id,
                timestamp: message.timestamp,
                hasData: !!message.data,
                keys: Object.keys(message.data)
              })
              
              // 🔥 REAL-TIME DEBUG: Track exact content changes
              if (content.trim() !== '') {
                console.log('📝 REAL-TIME CONTENT RECEIVED:', `"${content}"`)
              }
            }
            
            this.handleMessage(message)
          } catch (error) {
            console.error('❌ Error parsing WebSocket message:', error)
            console.error('Failed message data:', event.data)
          }
        }

        this.ws.onerror = (error) => {
          console.error('WebSocket connection error:', error)
          this.connectionPromise = null
          reject(error)
        }

        this.ws.onclose = (event) => {
          console.log('WebSocket connection closed', event)
          this.connectionPromise = null
          this.projectID = null
          this.connected = false
          this.handleReconnect()
        }
      } catch (error) {
        console.error('Failed to create WebSocket:', error)
        this.connectionPromise = null
        reject(error)
      }
    })

    return this.connectionPromise
  }

  disconnect() {
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
    this.projectID = null
    this.connectionPromise = null
    this.reconnectAttempts = 0
    // Reset connection state flag
    this.connected = false
  }

  sendMessage(type: string, data: any): void {
    if (!this.ws) {
      console.warn('⚠️ WebSocket not connected, cannot send message:', { type, data })
      return
    }

    const message: WebSocketMessage = {
      type,
      data,
      timestamp: Date.now(),
    }

    // Enhanced logging for all outgoing messages
    console.log('📤 SENDING WebSocket message:', {
      type,
      timestamp: message.timestamp,
      dataSize: JSON.stringify(data).length,
      hasContent: !!data.content,
      contentPreview: data.content
        ? data.content.substring(0, 100) + (data.content.length > 100 ? '...' : '')
        : null,
      conversationId: data.conversation_id,
      fullData: data,
    })

    // Special logging for user messages
    if (type === 'user_message' && data.content) {
      console.log('👤 USER MESSAGE SENT:', {
        conversationId: data.conversation_id,
        content: data.content,
        contentLength: data.content.length,
        timestamp: message.timestamp,
      })
    }

    // Log the raw JSON being sent
    const jsonString = JSON.stringify(message)
    console.log('🌐 RAW WebSocket payload:', {
      size: jsonString.length,
      payload: jsonString,
      timestamp: new Date().toISOString(),
    })

    this.ws.send(jsonString)
  }

  onMessage(type: string, handler: Function): void {
    this.messageHandlers.set(type, handler)
  }

  offMessage(type: string): void {
    this.messageHandlers.delete(type)
  }

  private getWebSocketURL(projectID: string): string {
    // Use Vite proxy for WebSocket connections in development
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsHost = window.location.host // Use dynamic host:port from Vite

    // Use proxied path - Vite will forward to backend
    console.log(
      `WebSocket URL: ${protocol}//${wsHost}/ws/chat?project=${encodeURIComponent(projectID)}`,
    )

    // Test if WebSocket server is accessible via HTTP first
    setTimeout(() => {
      const testUrl = `${protocol === 'wss:' ? 'https:' : 'http:'}//${wsHost}/ws/health`
      console.log(`Testing WebSocket proxy health: ${testUrl}`)
      fetch(testUrl)
        .then((response) => response.json())
        .then((data) => console.log('WebSocket proxy health:', data))
        .catch((error) => console.log('WebSocket proxy not accessible:', error))
    }, 1000)

    return `${protocol}//${wsHost}/ws/chat?project=${encodeURIComponent(projectID)}`
  }

  private handleMessage(message: WebSocketMessage): void {
    console.log('🎯 Processing WebSocket message:', {
      type: message.type,
      id: message.id,
      timestamp: message.timestamp,
      hasData: !!message.data,
      dataPreview: message.data ? JSON.stringify(message.data).substring(0, 200) + '...' : 'null',
    })

    // Ensure each message has a unique id
    if (!message.id) {
      message.id = `msg-${Date.now()}-${Math.random().toString(36).substr(2, 5)}`
      console.log('🏷️ Generated message ID:', message.id)
    }

    // Track tokens from responses
    if (message.data && typeof message.data === 'object') {
      if (message.data.tokens_used) {
        console.log('💰 Token usage detected:', message.data.tokens_used)
        this.trackTokenUsage(message.data.tokens_used)
      }

      // Also handle responses that include token usage in nested objects
      if (message.data.response && message.data.response.tokens_used) {
        console.log('💰 Nested token usage detected:', message.data.response.tokens_used)
        this.trackTokenUsage(message.data.response.tokens_used)
      }
    }

    const handler = this.messageHandlers.get(message.type)
    if (handler) {
      console.log('✅ Found handler for message type:', message.type)
      try {
        handler(message.data)
        console.log('✅ Handler executed successfully for:', message.type)
      } catch (handlerError) {
        console.error('❌ Handler execution failed for:', message.type, handlerError)
      }
    } else {
      console.log('⚠️ No handler found for WebSocket message:', message)
    }
  }

  private handleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.log('Max reconnection attempts reached')
      return
    }

    this.reconnectAttempts++
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1) // Exponential backoff

    console.log(
      `Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`,
    )

    setTimeout(() => {
      if (this.projectID) {
        this.connect(this.projectID).catch((error) => {
          console.error('Reconnection failed:', error)
        })
      }
    }, delay)
  }

  // Chat-specific methods
  sendMessageToAssistant(conversationID: string, content: string): void {
    this.sendMessage('user_message', {
      conversation_id: conversationID,
      content,
    })
  }

  createConversation(title?: string, initialMessage?: string): void {
    const data: any = {
      title: title || 'New Conversation',
    }

    if (initialMessage) {
      data.initial_message = initialMessage
    }

    this.sendMessage('create_conversation', data)
  }

  getConversations(): void {
    this.sendMessage('get_conversations', {})
  }

  getConversation(conversationID: string): void {
    this.sendMessage('get_conversation', {
      conversation_id: conversationID,
    })
  }

  deleteConversation(conversationID: string): void {
    this.sendMessage('delete_conversation', {
      conversation_id: conversationID,
    })
  }

  joinProject(projectID: string): void {
    this.sendMessage('join_project', {
      project_id: projectID,
    })
  }

  leaveProject(): void {
    this.sendMessage('leave_project', {})
  }

  // Request current streaming state for a conversation
  requestConversationStatus(conversationId: string): void {
    this.sendMessage('get_conversation_status', {
      conversation_id: conversationId,
    })
  }

  // 🔄 NEW: Load conversation including streaming state
  requestStreamingConversation(conversationId: string): void {
    this.sendMessage('get_streaming_conversation', {
      conversation_id: conversationId,
    })
  }

  // 🔄 NEW: Request all conversation statuses for persistence
  requestAllConversationStatuses(): void {
    this.sendMessage('get_all_conversation_statuses', {})
  }

  // Connection-level message handlers
  private setupConnectionHandlers() {
    // Ping for health check
    this.onMessage('ping', () => {
      this.sendMessage('pong', { timestamp: Date.now() })
    })
  }

  // Utility methods
  sendPing() {
    this.sendMessage('ping', {})
  }
  isConnected(): boolean {
    // Return true only if the internal flag is set and the underlying WebSocket is open
    return this.connected && this.ws !== null && this.ws.readyState === 1
  }

  // Token usage tracking methods
  getTokenUsage() {
    return {
      used: this.tokensUsed,
      limit: this.tokensLimit,
      remaining: this.tokensLimit - this.tokensUsed,
      percentage: (this.tokensUsed / this.tokensLimit) * 100,
    }
  }

  isTokenLimitExceeded() {
    return this.tokensUsed >= this.tokensLimit
  }

  setTokenLimit(limit: number) {
    this.tokensLimit = limit
  }

  resetTokenUsage() {
    this.tokensUsed = 0
  }

  // Private method to track tokens from responses
  private trackTokenUsage(tokens: number) {
    this.tokensUsed += tokens
    console.log(`Token usage updated: ${tokens} tokens used, total: ${this.tokensUsed}`)

    if (this.isTokenLimitExceeded()) {
      console.warn('Token limit exceeded!')
    }
  }

  getConnectionState(): string {
    if (!this.ws) return 'disconnected'

    switch (this.ws.readyState) {
      case WebSocket.CONNECTING:
        return 'connecting'
      case WebSocket.OPEN:
        return 'connected'
      case WebSocket.CLOSING:
        return 'closing'
      case WebSocket.CLOSED:
        return 'closed'
      default:
        return 'unknown'
    }
  }
}

// Singleton instance
export const webSocketService = new WebSocketService()

export default webSocketService
