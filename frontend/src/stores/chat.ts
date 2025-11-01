import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { ChatMessage, Conversation, ConversationDetails, ToolCall } from '@/services/websocket'
import webSocketService from '@/services/websocket'

export const useChatStore = defineStore('chat', () => {
  // State
  const conversations = ref<Map<string, Conversation>>(new Map())
  const currentConversationId = ref<string | null>(null)
  const messages = ref<ChatMessage[]>([])
  const isLoading = ref(false)
  const isConnected = ref(false)
  const connectionStatus = ref<string>('disconnected')

  // Computed
  const currentConversation = computed(() => {
    if (!currentConversationId.value) return null
    return conversations.value.get(currentConversationId.value) || null
  })

  const messageCount = computed(() => messages.value.length)

  const hasMessages = computed(() => messageCount.value > 0)

  // Actions
  const initWebSocket = async (projectId: string, token: string) => {
    try {
      await webSocketService.connect(projectId, token)
      isConnected.value = true
      connectionStatus.value = 'connected'
      
      // Set up message handlers
      setupMessageHandlers()
      
    } catch (error) {
      console.error('Failed to initialize WebSocket:', error)
      isConnected.value = false
      connectionStatus.value = 'error'
    }
  }

  const disconnectWebSocket = () => {
    webSocketService.disconnect()
    isConnected.value = false
    connectionStatus.value = 'disconnected'
  }

  const setupMessageHandlers = () => {
    // Assistant response
    webSocketService.onMessage('assistant_response', (data: any) => {
      if (data.content && data.content.trim()) {
        messages.value.push({
          id: data.message_id || `msg-${Date.now()}`,
          role: 'assistant',
          content: data.content,
          created_at: new Date(data.timestamp).toISOString(),
          metadata: data.metadata || {},
          tool_calls: data.tool_calls || [],
        })
      }
    })

    // Conversation created
    webSocketService.onMessage('conversation_created', (data: any) => {
      if (data.conversation) {
        conversations.value.set(data.conversation.id, data.conversation)
        currentConversationId.value = data.conversation.id
        messages.value = []
      }
    })

    // Conversations list
    webSocketService.onMessage('conversations_list', (data: any) => {
      if (data.conversations && Array.isArray(data.conversations)) {
        data.conversations.forEach((conv: Conversation) => {
          conversations.value.set(conv.id, conv)
        })
      }
    })

    // Conversation details
    webSocketService.onMessage('conversation_details', (data: any) => {
      if (data.conversation) {
        const { conversation, messages: convMessages } = data.conversation as ConversationDetails
        conversations.value.set(conversation.id, conversation)
        
        if (convMessages && Array.isArray(convMessages)) {
          messages.value = convMessages
        }
        
        currentConversationId.value = conversation.id
      }
    })

    // Conversation deleted
    webSocketService.onMessage('conversation_deleted', (data: any) => {
      if (data.conversation_id) {
        conversations.value.delete(data.conversation_id)
        
        if (currentConversationId.value === data.conversation_id) {
          currentConversationId.value = null
          messages.value = []
        }
      }
    })

    // User message sent confirmation
    webSocketService.onMessage('user_message_sent', (data: any) => {
      if (data.message) {
        messages.value.push(data.message)
      }
    })

    // Tool execution
    webSocketService.onMessage('tool_execution', (data: any) => {
      // Update tool call status in existing messages
      if (data.message_id && data.tool_index !== undefined) {
        const messageIndex = messages.value.findIndex(msg => msg.id === data.message_id)
        if (messageIndex !== -1 && messages.value[messageIndex].tool_calls) {
          messages.value[messageIndex].tool_calls![data.tool_index] = data.tool_call
        }
      }
    })

    // Tool execution started
    webSocketService.onMessage('tool_execution_started', (data: any) => {
      console.log('Tool execution started:', data)
    })

    // Tool execution completed
    webSocketService.onMessage('tool_execution_completed', (data: any) => {
      console.log('Tool execution completed:', data)
    })

    // Tool execution failed
    webSocketService.onMessage('tool_execution_failed', (data: any) => {
      console.log('Tool execution failed:', data)
    })

    // Project joined
    webSocketService.onMessage('project_joined', (data: any) => {
      console.log('Project joined:', data)
    })

    // Pong response
    webSocketService.onMessage('pong', (data: any) => {
      console.log('Pong received:', data)
    })

    // Error handling
    webSocketService.onMessage('error', (data: any) => {
      console.error('WebSocket error:', data)
      isLoading.value = false
    })
  }

  const sendMessage = async (content: string) => {
    if (!content.trim() || !currentConversationId.value) return

    // Create user message immediately for better UX
    const userMessage: ChatMessage = {
      id: `msg-${Date.now()}`,
      role: 'user',
      content: content.trim(),
      created_at: new Date().toISOString(),
    }

    messages.value.push(userMessage)
    isLoading.value = true

    // Send to WebSocket
    webSocketService.sendMessageToAssistant(currentConversationId.value, content)
  }

  const createConversation = async (title?: string) => {
    isLoading.value = true
    webSocketService.createConversation(title)
    // Response will be handled by message handlers
  }

  const loadConversations = () => {
    webSocketService.getConversations()
  }

  const loadConversation = (conversationId: string) => {
    webSocketService.getConversation(conversationId)
  }

  const deleteConversation = (conversationId: string) => {
    webSocketService.deleteConversation(conversationId)
  }

  const selectConversation = (conversationId: string) => {
    if (conversations.value.has(conversationId)) {
      currentConversationId.value = conversationId
      loadConversation(conversationId)
    }
  }

  const clearCurrentConversation = () => {
    currentConversationId.value = null
    messages.value = []
  }

  // Formatting helpers
  const formatMessageTime = (message: ChatMessage) => {
    return new Date(message.created_at).toLocaleString()
  }

  const isToolMessage = (message: ChatMessage) => {
    return message.tool_calls && message.tool_calls.length > 0
  }

  const getToolStatus = (toolCall: ToolCall) => {
    return toolCall.status || 'pending'
  }

  return {
    // State
    conversations,
    currentConversationId,
    currentConversation,
    messages,
    isLoading,
    isConnected,
    connectionStatus,

    // Computed
    messageCount,
    hasMessages,

    // Actions
    initWebSocket,
    disconnectWebSocket,
    sendMessage,
    createConversation,
    loadConversations,
    loadConversation,
    deleteConversation,
    selectConversation,
    clearCurrentConversation,

    // Helpers
    formatMessageTime,
    isToolMessage,
    getToolStatus,
  }
})
