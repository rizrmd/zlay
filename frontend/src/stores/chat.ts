import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import type { ChatMessage, ToolCall } from '@/services/websocket'
import webSocketService from '@/services/websocket'
import { useConversationStore } from './conversation'
import { useProjectStore } from './project'

export const useChatStore = defineStore('chat', () => {
  // Router for navigation
  const router = useRouter()
  
  // Inject other stores
  const conversationStore = useConversationStore()
  const projectStore = useProjectStore()
  
  // Chat-specific state
  const streamingState = ref<Map<string, any>>(new Map())
  const toolStatuses = ref<Map<string, string>>(new Map())
  
  // Loading states specific to chat operations
  const isSendingMessage = ref(false)
  const isCreatingConversation = ref(false)
  
  // Computed - delegate to specialized stores
  const conversations = computed(() => conversationStore.conversations)
  const currentConversationId = computed(() => conversationStore.currentConversationId)
  const currentConversation = computed(() => conversationStore.currentConversation)
  const messages = computed(() => conversationStore.messages)
  const currentConversationMessages = computed(() => conversationStore.currentConversationMessages)
  const messageCount = computed(() => conversationStore.messageCount)
  const hasMessages = computed(() => conversationStore.hasMessages)
  const isLoading = computed(() => isSendingMessage.value || isCreatingConversation.value || conversationStore.isProcessing)
  const isConnected = computed(() => projectStore.isConnected)
  const connectionStatus = computed(() => projectStore.connectionStatus)
  const processingConversations = computed(() => conversationStore.processingConversations)
  const isLoadingHistory = computed(() => conversationStore.isLoadingHistory)
  const isProcessing = computed(() => conversationStore.isProcessing)
  const canChat = computed(() => !isLoading.value && !isLoadingHistory.value && isConnected.value)
  const anyConversationProcessing = computed(() => conversationStore.anyConversationProcessing)
  
  // Chat-specific actions
  const setupMessageHandlers = () => {
    console.log('🚀 CHAT STORE: Setting up message handlers')
    
    // Assistant response - handle streaming messages
    webSocketService.onMessage('assistant_response', (data: any) => {
      const messageId = data.message_id || `msg-${Date.now()}`
      const conversationId = data.conversation_id || currentConversationId.value
      
      console.log('🔍 CHECKING PROCESSING STATUS:', {
        conversationId,
        isProcessing: processingConversations.value.has(conversationId),
        allProcessing: Array.from(processingConversations.value),
        currentConversation: currentConversationId.value,
      })
      
      if (!processingConversations.value.has(conversationId)) {
        console.log('❌ DEBUG: Ignoring message for untracked conversation:', conversationId)
        return
      }
      
      console.log('✅ DEBUG: Message will be processed for conversation:', conversationId)
      
      // Process content for streaming chunks
      if (data.content !== undefined && typeof data.content === 'string') {
        console.log('📝 DEBUG: Processing content chunk:', data.content.trim() || '[empty]')
        
        // Always process the message (even empty content) to ensure message exists
        conversationStore.addOrUpdateMessage(conversationId, {
          id: messageId,
          conversation_id: data.conversation_id || currentConversationId.value,
          role: 'assistant',
          content: data.content, // Pass the actual chunk (could be empty)
          created_at: data.timestamp ? new Date(data.timestamp).toISOString() : new Date().toISOString(),
          metadata: data.metadata || {},
          tool_calls: data.tool_calls || [],
        })
      }
      
      // Handle streaming completion
      if (data.done === true) {
        conversationStore.removeFromProcessing(conversationId)
        
        // Mark message as complete
        const convMessages = conversationStore.conversationMessages.value.get(conversationId) || []
        const messageIndex = convMessages.findIndex((msg) => msg.id === messageId)
        
        if (messageIndex !== -1) {
          const msg = convMessages[messageIndex]
          if (msg) {
            const updatedMessages = [...convMessages]
            updatedMessages[messageIndex] = {
              ...msg,
              metadata: {
                ...msg.metadata,
                done: true,
                completed_at: new Date().toISOString(),
              },
            }
            conversationStore.conversationMessages.value.set(conversationId, updatedMessages)
          }
        }
        
        // Reset sending state when streaming is complete
        if (conversationId === currentConversationId.value) {
          isSendingMessage.value = false
        }
      }
    })
    
    // Conversation created
    webSocketService.onMessage('conversation_created', (data: any) => {
      if (data.conversation) {
        conversationStore.conversations.value.set(data.conversation.id, data.conversation)
        conversationStore.currentConversationId.value = data.conversation.id
        conversationStore.messages.value = []
        
        // Reset loading states
        isCreatingConversation.value = false
        isSendingMessage.value = false
        
        // Auto-navigate to conversation URL
        const projectId = projectStore.currentProjectId
        if (projectId) {
          console.log('🚀 Navigating to new conversation URL:', `/p/${projectId}/chat/${data.conversation.id}`)
          router.replace(`/p/${projectId}/chat/${data.conversation.id}`)
        }
      }
    })
    
    // Conversations list
    webSocketService.onMessage('conversations_list', (data: any) => {
      console.log('DEBUG: Received conversations list:', data)
      if (data.conversations && Array.isArray(data.conversations)) {
        data.conversations.forEach((conv: any) => {
          conversationStore.conversations.value.set(conv.id, conv)
        })
        console.log('DEBUG: Loaded conversations:', Array.from(conversationStore.conversations.value.keys()))
      }
    })
    
    // Conversation details
    webSocketService.onMessage('conversation_details', (data: any) => {
      console.log('DEBUG: Received conversation details:', data)
      if (data.conversation) {
        const conversationWithMessages = data.conversation
        const conversation = conversationWithMessages.conversation
        const convMessages = conversationWithMessages.messages
        
        conversationStore.conversations.value.set(conversation.id, conversation)
        
        if (convMessages && Array.isArray(convMessages)) {
          const safeMessages = convMessages.map((msg: any) => {
            if (!msg.id) {
              msg.id = `msg-${Date.now()}-${Math.random().toString(36).substr(2, 5)}`
            }
            return msg
          })
          conversationStore.messages.value = safeMessages
          conversationStore.conversationMessages.value.set(conversation.id, safeMessages)
        }
        
        conversationStore.currentConversationId.value = conversation.id
      }
    })
    
    // Conversation deleted
    webSocketService.onMessage('conversation_deleted', (data: any) => {
      if (data.conversation_id) {
        conversationStore.conversations.value.delete(data.conversation_id)
        
        if (currentConversationId.value === data.conversation_id) {
          conversationStore.clearCurrentConversation()
        }
      }
    })
    
    // User message sent confirmation
    webSocketService.onMessage('user_message_sent', (data: any) => {
      if (data.message) {
        const msg = data.message
        if (!msg.id) {
          msg.id = `msg-${Date.now()}`
        }
        
        // Check if message already exists
        const existingIndex = messages.value.findIndex(
          (m) => m.role === 'user' && m.content === msg.content
        )
        
        if (existingIndex === -1) {
          conversationStore.addOrUpdateMessage(msg.conversation_id, msg)
        }
      }
    })
    
    // Tool execution status updates
    webSocketService.onMessage('tool_execution_started', (data: any) => {
      toolStatuses.value.set(data.tool_call_id, 'executing')
    })
    
    webSocketService.onMessage('tool_execution_completed', (data: any) => {
      toolStatuses.value.set(data.tool_call_id, 'completed')
      updateToolCallStatus(data.tool_call_id, data.message_id, 'completed', data.result)
    })
    
    webSocketService.onMessage('tool_execution_failed', (data: any) => {
      toolStatuses.value.set(data.tool_call_id, 'failed')
      updateToolCallStatus(data.tool_call_id, data.message_id, 'failed', data.error)
    })
    
    // Chat interruption
    webSocketService.onMessage('chat_interrupted', (data: any) => {
      if (currentConversationId.value) {
        conversationStore.loadConversation(currentConversationId.value)
      }
    })
    
    // Error handling
    webSocketService.onMessage('error', (data: any) => {
      console.error('WebSocket error:', data)
      isSendingMessage.value = false
      isCreatingConversation.value = false
    })
  }
  
  // Chat actions
  const sendMessage = async (content: string) => {
    if (!content.trim() || !currentConversationId.value || isSendingMessage.value) return
    
    // Add conversation to processing set
    conversationStore.addToProcessing(currentConversationId.value)
    
    // Create user message
    const userMessage: ChatMessage = {
      id: `msg-${Date.now()}`,
      conversation_id: currentConversationId.value || '',
      role: 'user',
      content: content.trim(),
      created_at: new Date().toISOString(),
    }
    
    conversationStore.addOrUpdateMessage(currentConversationId.value, userMessage)
    isSendingMessage.value = true
    
    // Send to WebSocket
    webSocketService.sendMessageToAssistant(currentConversationId.value, content)
  }
  
  const createConversation = async (title?: string, initialMessage?: string) => {
    isCreatingConversation.value = true
    webSocketService.createConversation(title, initialMessage)
  }
  
  const deleteConversation = (conversationId: string) => {
    if (confirm('Are you sure you want to delete this conversation?')) {
      conversationStore.deleteConversation(conversationId)
    }
  }
  
  const selectConversation = (conversationId: string) => {
    conversationStore.selectConversation(conversationId)
  }
  
  // Tool status management
  const updateToolCallStatus = (
    toolCallId: string,
    messageIdOrConvId: string,
    status: string,
    result?: any,
  ) => {
    const messageIndex = messages.value.findIndex(
      (msg) => msg.id === messageIdOrConvId || 
        (msg.tool_calls && msg.tool_calls.some((tc) => tc.id === toolCallId))
    )
    
    if (messageIndex !== -1 && messages.value[messageIndex]?.tool_calls) {
      const toolCalls = messages.value[messageIndex]?.tool_calls
      if (toolCalls) {
        const toolCallIndex = toolCalls.findIndex((tc) => tc.id === toolCallId)
        if (toolCallIndex !== -1) {
          const toolCall = toolCalls[toolCallIndex]
          if (toolCall) {
            toolCall.status = status
            if (status === 'completed' && result) {
              toolCall.result = result
            } else if (status === 'failed' && result) {
              toolCall.error = result
            }
          }
        }
      }
    }
  }
  
  // Handle base chat path redirection
  const handleBaseChatPath = async (projectId: string) => {
    console.log('🚀 Handling base chat path for project:', projectId)
    
    try {
      await conversationStore.loadConversations()
      
      const projectConversations = Array.from(conversationStore.conversations.value.values())
        .filter((conv: any) => conv.project_id === projectId)
        .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
      
      if (projectConversations.length > 0) {
        const latestConversation = projectConversations[0]
        console.log('🚀 Redirecting to latest conversation:', latestConversation.id)
        router.replace(`/p/${projectId}/chat/${latestConversation.id}`)
      } else {
        console.log('🚀 No conversations found, creating new one')
        await createConversation()
      }
    } catch (error) {
      console.error('🚀 Error handling base chat path:', error)
      await createConversation()
    }
  }
  
  // Initialize chat store
  const initChat = async (projectId: string) => {
    console.log('🚀 CHAT STORE: Initializing chat for project:', projectId)
    
    // Initialize WebSocket connection (handled by project store)
    await projectStore.initWebSocket(projectId)
    
    // Setup message handlers
    setupMessageHandlers()
    
    // Load conversations
    await conversationStore.loadConversations()
  }
  
  return {
    // State (computed from specialized stores)
    conversations,
    currentConversationId,
    currentConversation,
    conversationMessages: computed(() => conversationStore.conversationMessages),
    messages,
    isLoading,
    isConnected,
    connectionStatus,
    processingConversations,
    isLoadingHistory,
    isSendingMessage,
    isCreatingConversation,
    
    // Computed
    messageCount,
    hasMessages,
    currentConversationMessages,
    isProcessing,
    canChat,
    anyConversationProcessing,
    
    // Chat-specific state
    streamingState,
    toolStatuses,
    
    // Actions
    initChat,
    sendMessage,
    createConversation,
    deleteConversation,
    selectConversation,
    handleBaseChatPath,
    clearCurrentConversation: conversationStore.clearCurrentConversation,
    loadConversations: conversationStore.loadConversations,
    loadConversation: conversationStore.loadConversation,
    
    // Tool management
    updateToolCallStatus,
    getToolStatus: (toolCall: ToolCall) => toolCall.status || 'pending',
    isToolMessage: (message: ChatMessage) => message.tool_calls && message.tool_calls.length > 0,
    formatMessageTime: (message: ChatMessage) => new Date(message.created_at).toLocaleString(),
  }
})