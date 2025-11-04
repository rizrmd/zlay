import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
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

  // ✅ NEW: Track streaming message IDs per conversation for consistent accumulation
  const streamingMessageIds = ref<Map<string, string>>(new Map())

  // ✅ CORE LOGIC: Check if current conversation is processing based ONLY on conversation.status
  const isCurrentConversationProcessing = computed(() => {
    const conv = conversationStore.currentConversationId
      ? conversationStore.conversations.get(conversationStore.currentConversationId)
      : null
    const isProcessing = conv?.status === 'processing'

    console.log('💬 CHECKING:', {
      conversationId: conversationStore.currentConversationId,
      conversationStatus: conv?.status,
      isProcessing,
    })

    return isProcessing || false
  })

  const canChat = computed(() => {
    return !isCurrentConversationProcessing.value && projectStore.isConnected
  })

  // Setup message handlers
  const setupMessageHandlers = () => {
    console.log('🚀 CHAT STORE: Setting up message handlers')

    // Assistant response streaming
    webSocketService.onMessage('assistant_response', (data: any) => {
      const conversationId = data.conversation_id || conversationStore.currentConversationId

      if (!conversationId) {
        console.log('❌ No conversation ID, ignoring message')
        return
      }

      // 🔍 DEBUG: Log every incoming streaming chunk
      console.log('📥 STREAMING CHUNK RECEIVED:', {
        conversationId,
        backendMessageId: data.message_id,
        content: data.content,
        contentLength: data.content?.length || 0,
        done: data.done,
        hasExistingStreamingId: streamingMessageIds.value.has(conversationId),
        currentStreamingId: streamingMessageIds.value.get(conversationId),
        timestamp: Date.now(),
      })

      // ✅ FIXED: Use consistent message ID for streaming sessions
      let messageId = data.message_id
      if (!messageId) {
        // Check if we already have a streaming session for this conversation
        if (streamingMessageIds.value.has(conversationId)) {
          messageId = streamingMessageIds.value.get(conversationId)
          console.log(
            '🔄 Using existing streaming message ID:',
            messageId,
            'for conversation:',
            conversationId,
          )
        } else {
          // Create new streaming message ID and track it
          messageId = `stream-${conversationId}-${Date.now()}`
          streamingMessageIds.value.set(conversationId, messageId)
          console.log(
            '🆕 Creating new streaming message ID:',
            messageId,
            'for conversation:',
            conversationId,
          )
        }
      } else {
        // Backend provided message_id - check if we should track it
        if (!streamingMessageIds.value.has(conversationId)) {
          streamingMessageIds.value.set(conversationId, messageId)
          console.log(
            '📋 Tracking backend-provided message ID:',
            messageId,
            'for conversation:',
            conversationId,
          )
        } else {
          const existingId = streamingMessageIds.value.get(conversationId)
          if (existingId !== messageId) {
            console.warn(
              '⚠️ Message ID mismatch! Existing:',
              existingId,
              'New:',
              messageId,
              'This could cause issues!',
            )
          }
        }
      }

      // Process streaming chunks
      if (data.content !== undefined && typeof data.content === 'string') {
        console.log('📝 PROCESSING CONTENT:', {
          messageId,
          conversationId,
          contentPreview: data.content.substring(0, 50) + (data.content.length > 50 ? '...' : ''),
          fullContent: `"${data.content}"`,
          contentLength: data.content.length,
        })

        conversationStore.addOrUpdateMessage(conversationId, {
          id: messageId,
          conversation_id: conversationId,
          role: 'assistant',
          content: data.content,
          created_at: data.timestamp
            ? new Date(data.timestamp).toISOString()
            : new Date().toISOString(),
          metadata: data.metadata || {},
          tool_calls: data.tool_calls || [],
        })
      }

      // Handle streaming completion
      if (data.done === true) {
        // ✅ CLEANUP: Remove streaming message ID when done
        streamingMessageIds.value.delete(conversationId)
        console.log('🧹 Cleaned up streaming message ID for conversation:', conversationId)

        // ✅ UPDATE: Set conversation status to 'completed'
        const conv = conversationStore.conversations.get(conversationId)
        if (conv && conv.status !== 'completed') {
          const updatedConv = { ...conv, status: 'completed' }
          conversationStore.conversations.set(conversationId, updatedConv)
          console.log('💬 STREAMING COMPLETE: Status updated to completed:', conversationId)
        }
      }
    })

    // Conversation created
    webSocketService.onMessage('conversation_created', (data: any) => {
      if (data.conversation) {
        conversationStore.conversations.set(data.conversation.id, data.conversation)
        conversationStore.currentConversationId = data.conversation.id
        conversationStore.messages = []

        const projectId = projectStore.currentProjectId
        if (projectId) {
          router.replace(`/p/${projectId}/chat/${data.conversation.id}`)
        }
      }
    })

    // Error handling
    webSocketService.onMessage('error', (data: any) => {
      console.error('WebSocket error:', data)
      // Reset conversation status on error
      if (conversationStore.currentConversationId) {
        const conv = conversationStore.conversations.get(conversationStore.currentConversationId)
        if (conv && conv.status === 'processing') {
          const updatedConv = { ...conv, status: 'completed' }
          conversationStore.conversations.set(conversationStore.currentConversationId, updatedConv)
          console.log('💬 ERROR: Reset status to completed')
        }
        // ✅ CLEANUP: Clean up streaming message ID on error
        streamingMessageIds.value.delete(conversationStore.currentConversationId)
        console.log(
          '🧹 Cleaned up streaming message ID due to error:',
          conversationStore.currentConversationId,
        )
      }
    })
  }

  // Send message
  const sendMessage = async (content: string) => {
    if (!content.trim() || !conversationStore.currentConversationId) return

    // ✅ UPDATE: Set conversation status to 'processing'
    const conv = conversationStore.conversations.get(conversationStore.currentConversationId)
    if (conv && conv.status !== 'processing') {
      const updatedConv = { ...conv, status: 'processing' }
      conversationStore.conversations.set(conversationStore.currentConversationId, updatedConv)
      console.log('💬 SEND: Status updated to processing:', conversationStore.currentConversationId)
    }

    // Create and add user message
    const userMessage: ChatMessage = {
      id: `msg-${Date.now()}`,
      conversation_id: conversationStore.currentConversationId,
      role: 'user',
      content: content.trim(),
      created_at: new Date().toISOString(),
    }

    conversationStore.addOrUpdateMessage(conversationStore.currentConversationId, userMessage)
    webSocketService.sendMessageToAssistant(conversationStore.currentConversationId, content)
  }

  // Create conversation
  const createConversation = async (title?: string, initialMessage?: string) => {
    webSocketService.createConversation(title, initialMessage)
  }

  // Delete conversation
  const deleteConversation = (conversationId: string) => {
    if (confirm('Are you sure you want to delete this conversation?')) {
      // ✅ CLEANUP: Clean up streaming state for deleted conversation
      streamingMessageIds.value.delete(conversationId)
      streamingState.value.delete(conversationId)
      console.log('🧹 Cleaned up streaming state for deleted conversation:', conversationId)

      conversationStore.deleteConversation(conversationId)
    }
  }

  // Select conversation
  const selectConversation = (conversationId: string) => {
    conversationStore.selectConversation(conversationId)
  }

  // ✅ NEW: Function to manually interrupt streaming for a conversation
  const interruptStreaming = (conversationId?: string) => {
    const targetConversationId = conversationId || conversationStore.currentConversationId
    if (targetConversationId) {
      // Clean up streaming state
      streamingMessageIds.value.delete(targetConversationId)
      streamingState.value.delete(targetConversationId)

      // Reset conversation status if it's processing
      const conv = conversationStore.conversations.get(targetConversationId)
      if (conv && conv.status === 'processing') {
        const updatedConv = { ...conv, status: 'completed' }
        conversationStore.conversations.set(targetConversationId, updatedConv)
        console.log(
          '💬 INTERRUPT: Reset status to completed for conversation:',
          targetConversationId,
        )
      }

      console.log('🛑 Interrupted streaming for conversation:', targetConversationId)
    }
  }

  // Handle base chat path redirection
  const handleBaseChatPath = async (projectId: string) => {
    console.log('🚀 Handling base chat path for project:', projectId)

    try {
      await conversationStore.loadConversations()

      const projectConversations = Array.from(conversationStore.conversations.values())
        .filter((conv: any) => conv.project_id === projectId)
        .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())

      if (projectConversations.length > 0) {
        const latestConversation = projectConversations[0]
        if (latestConversation) {
          router.replace(`/p/${projectId}/chat/${latestConversation.id}`)
        }
      } else {
        await createConversation()
      }
    } catch (error) {
      console.error('🚀 Error handling base chat path:', error)
      await createConversation()
    }
  }

  // Initialize chat
  const initChat = async (projectId: string) => {
    console.log('🚀 CHAT STORE: Initializing chat for project:', projectId)

    await projectStore.initWebSocket(projectId)
    setupMessageHandlers()
    await conversationStore.loadConversations()
  }

  // Tool management
  const updateToolCallStatus = (
    toolCallId: string,
    messageIdOrConvId: string,
    status: string,
    result?: any,
  ) => {
    const messages = conversationStore.messages.value
    const messageIndex = messages.findIndex(
      (msg) =>
        msg.id === messageIdOrConvId ||
        (msg.tool_calls && msg.tool_calls.some((tc: any) => tc.id === toolCallId)),
    )

    if (messageIndex !== -1 && messages[messageIndex]?.tool_calls) {
      const toolCalls = messages[messageIndex]?.tool_calls as any[]
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

  return {
    // State (from specialized stores)
    conversations: computed(() => conversationStore.conversations),
    currentConversationId: computed(() => conversationStore.currentConversationId),
    currentConversation: computed(() => conversationStore.currentConversation),
    messages: computed(() => conversationStore.messages),
    isLoading: computed(() => conversationStore.isLoading),
    isLoadingHistory: computed(() => conversationStore.isLoadingHistory),
    isLoadingConversations: computed(() => conversationStore.isLoadingConversations),
    isConnected: computed(() => projectStore.isConnected),
    connectionStatus: computed(() => projectStore.connectionStatus),

    // ✅ CORE LOGIC
    isCurrentConversationProcessing,
    canChat,

    // Chat-specific state
    streamingState,
    toolStatuses,
    streamingMessageIds,

    // Actions
    initChat,
    sendMessage,
    createConversation,
    deleteConversation,
    selectConversation,
    interruptStreaming,
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
