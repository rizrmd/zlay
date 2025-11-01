export interface WebSocketMessage {
  type: string
  data: any
  timestamp?: number
  id?: string
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
  private connectionPromise: Promise<void> | null = null

  async connect(projectID: string, token: string): Promise<void> {
    // If already connecting, wait for existing connection
    if (this.connectionPromise) {
      return this.connectionPromise
    }

    this.connectionPromise = new Promise<void>((resolve, reject) => {
      try {
        // Determine WebSocket URL
        const wsUrl = this.getWebSocketURL(projectID, token)
        
        this.ws = new WebSocket(wsUrl)

        this.ws.onopen = () => {
          console.log('WebSocket connected')
          this.projectID = projectID
          this.reconnectAttempts = 0
          this.reconnectDelay = 1000
          resolve()
        }

        this.ws.onmessage = (event) => {
          try {
            const message = JSON.parse(event.data) as WebSocketMessage
            this.handleMessage(message)
          } catch (error) {
            console.error('Error parsing WebSocket message:', error)
          }
        }

        this.ws.onerror = (error) => {
          console.error('WebSocket error:', error)
          this.connectionPromise = null
          reject(error)
        }

        this.ws.onclose = (event) => {
          console.log('WebSocket disconnected', event)
          this.connectionPromise = null
          this.projectID = null
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
  }

  sendMessage(type: string, data: any): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn('WebSocket not connected, cannot send message')
      return
    }

    const message: WebSocketMessage = {
      type,
      data,
      timestamp: Date.now(),
    }

    this.ws.send(JSON.stringify(message))
  }

  onMessage(type: string, handler: Function): void {
    this.messageHandlers.set(type, handler)
  }

  offMessage(type: string): void {
    this.messageHandlers.delete(type)
  }

  private getWebSocketURL(projectID: string, token: string): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsHost = window.location.hostname === 'localhost' ? 'localhost:6070' : window.location.host
    
    return `${protocol}//${wsHost}/ws/chat?token=${encodeURIComponent(token)}&project=${encodeURIComponent(projectID)}`
  }

  private handleMessage(message: WebSocketMessage): void {
    const handler = this.messageHandlers.get(message.type)
    if (handler) {
      handler(message.data)
    } else {
      console.log('Unhandled WebSocket message:', message)
    }
  }

  private handleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.log('Max reconnection attempts reached')
      return
    }

    this.reconnectAttempts++
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1) // Exponential backoff
    
    console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`)
    
    setTimeout(() => {
      if (this.projectID) {
        this.connect(this.projectID, this.getAuthToken()).catch(error => {
          console.error('Reconnection failed:', error)
        })
      }
    }, delay)
  }

  private getAuthToken(): string {
    // Get auth token from cookies or local storage
    const cookies = document.cookie.split(';')
    for (const cookie of cookies) {
      const [name, value] = cookie.trim().split('=')
      if (name === 'auth_token') {
        return value
      }
    }
    
    // Fallback to local storage
    return localStorage.getItem('auth_token') || ''
  }

  // Chat-specific methods
  sendMessageToAssistant(conversationID: string, content: string): void {
    this.sendMessage('user_message', {
      conversation_id: conversationID,
      content,
    })
  }

  createConversation(title?: string): void {
    this.sendMessage('create_conversation', {
      title: title || 'New Conversation',
    })
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

  // Utility methods
  isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN
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
