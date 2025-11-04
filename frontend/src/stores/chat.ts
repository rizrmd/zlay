import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import type { ChatMessage, Conversation, ConversationDetails, ToolCall } from '@/services/websocket'
import type { Conversation as ApiConversation, ApiMessage } from '@/services/api'
import webSocketService from '@/services/websocket'
import { apiClient } from '@/services/api'

export const useChatStore = defineStore('chat', () => {
  // State
  const conversations = ref<Map<string, Conversation>>(new Map())
  const currentConversationId = ref<string | null>(null)
  const allMessages = ref<ChatMessage[]>([]) // Legacy, for compatibility
  const isLoading = ref(false)
  const isConnected = ref(false)
  const connectionStatus = ref<string>('disconnected')

  // Track which conversations are currently being processed (multiple can be processing)
  const processingConversations = ref<Set<string>>(new Set())

  // Store messages per conversation for independent streaming
  const conversationMessages = ref<Map<string, ChatMessage[]>>(new Map())

  // Track if we're currently loading conversation history
  const isLoadingHistory = ref(false)

  // Computed
  const currentConversation = computed(() => {
    if (!currentConversationId.value) return null
    return conversations.value.get(currentConversationId.value) || null
  })

  const messageCount = computed(() => currentConversationMessages.value.length)

  const hasMessages = computed(() => messageCount.value > 0)

  // Computed for current conversation messages - keeps UI messages separate from processing
  const currentConversationMessages = computed(() => {
    if (!currentConversationId.value) return []
    return conversationMessages.value.get(currentConversationId.value) || []
  })

  // Legacy messages for UI - use ref for simplicity
  const messages = ref<ChatMessage[]>([])

  // Debug messages state
  watch(messages, (newMessages) => {
    console.log('DEBUG: Messages ref updated, length:', newMessages?.length)
  })

  // Watch for current conversation changes and sync legacy messages
  // Update messages when current conversation changes
  watch(
    () => currentConversationId.value,
    (newConversationId) => {
      if (newConversationId) {
        const convMessages = conversationMessages.value.get(newConversationId) || []
        messages.value = [...convMessages]
      } else {
        messages.value = []
      }
    },
    { immediate: true },
  )

  // Computed for loading state
  const isProcessing = computed(() => isLoading.value || isLoadingHistory.value)
  const canChat = computed(() => !isLoading.value && !isLoadingHistory.value && isConnected.value)
  const anyConversationProcessing = computed(() => processingConversations.value.size > 0)

  // Actions
  const initWebSocket = async (projectId: string) => {
    console.log('initWebSocket called with:', { projectId })

    try {
      await webSocketService.connect(projectId)
      isConnected.value = true
      connectionStatus.value = 'connected'

      // Set up message handlers
      setupMessageHandlers()

      console.log('WebSocket initialization successful')
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
      const messageId = data.message_id || `msg-${Date.now()}`
      const conversationId = data.conversation_id || currentConversationId.value

      // Only process messages for conversations that are being processed
      console.log('🔍 CHECKING PROCESSING STATUS:', {
        conversationId,
        isProcessing: processingConversations.value.has(conversationId),
        allProcessing: Array.from(processingConversations.value),
        currentConversation: currentConversationId.value,
      })

      if (!processingConversations.value.has(conversationId)) {
        console.log(
          '❌ DEBUG: Ignoring message for untracked conversation:',
          conversationId,
          'Processing conversations:',
          Array.from(processingConversations.value),
        )
        return
      }

      console.log('✅ DEBUG: Message will be processed for conversation:', conversationId)

      // Always store messages, but only update UI for current conversation
      if (conversationId === currentConversationId.value) {
        console.log('DEBUG: Processing message for current conversation:', conversationId)
      } else {
        console.log('DEBUG: Storing message for background conversation:', conversationId)
      }

      // Only process content for streaming chunks (not the final completion)
      if (data.content && typeof data.content === 'string') {
        // Only accumulate content if it's not empty
        if (data.content.trim() !== '') {
          console.log('📝 DEBUG: Processing non-empty content:', data.content)
          // Get or create messages array for this conversation
          const convMessages = conversationMessages.value.get(conversationId) || []
          const existingMessageIndex = convMessages.findIndex((msg) => msg.id === messageId)

          if (existingMessageIndex !== -1) {
            // Update existing message (streaming) - ensure Vue reactivity
            const msg = convMessages[existingMessageIndex]
            if (msg) {
              // 🔥 REAL-TIME FIX: Create new message with appended content
              const updatedMsg = {
                ...msg,
                content: msg.content + data.content,
              }

              // Create completely new array for max reactivity
              const updatedMessages = [...convMessages]
              updatedMessages[existingMessageIndex] = updatedMsg

              // Update conversation store
              conversationMessages.value.set(conversationId, updatedMessages)

              // 🔥 CRITICAL: Also update UI messages if this is current conversation
              if (conversationId === currentConversationId.value) {
                const oldMessages = [...messages.value]
                messages.value = [...updatedMessages]
                console.log('🔄 UI SYNC - EXISTING MESSAGE UPDATED:', {
                  conversationId,
                  messageId,
                  oldMessagesCount: oldMessages.length,
                  newMessagesCount: messages.value.length,
                  lastMessageContent: messages.value[messages.value.length - 1]?.content,
                })
              }

              console.log('📝 REAL-TIME STREAM UPDATE:', {
                conversationId,
                messageId,
                oldContent: msg.content,
                newContent: data.content,
                totalContent: updatedMsg.content,
                totalLength: updatedMsg.content.length,
              })
            }
          } else {
            // Create new assistant message for streaming
            const newMessage: ChatMessage = {
              id: messageId,
              conversation_id: data.conversation_id || currentConversationId.value,
              role: 'assistant',
              content: data.content,
              created_at: data.timestamp
                ? typeof data.timestamp === 'number'
                  ? new Date(data.timestamp).toISOString()
                  : data.timestamp
                : new Date().toISOString(),
              metadata: data.metadata || {},
              tool_calls: data.tool_calls || [],
            }

            // Create new array with added message
            const updatedMessages = [...convMessages, newMessage]
            conversationMessages.value.set(conversationId, updatedMessages)

            // 🔥 CRITICAL: Update UI messages if this is current conversation
            if (conversationId === currentConversationId.value) {
              const oldMessages = [...messages.value]
              messages.value = [...updatedMessages]
              console.log('🔄 UI SYNC - NEW MESSAGE ADDED:', {
                conversationId,
                messageId,
                oldMessagesCount: oldMessages.length,
                newMessagesCount: messages.value.length,
                lastMessageContent: messages.value[messages.value.length - 1]?.content,
              })
            }

            console.log('📝 REAL-TIME NEW MESSAGE:', {
              conversationId,
              messageId,
              content: data.content,
              contentLength: data.content.length,
              totalMessages: updatedMessages.length,
            })
          }
        } // 🔥 CLOSING BRACE for empty content check
      }

      // Handle streaming completion (when done: true) - don't add content, just mark as complete
      if (data.done === true) {
        // Remove from processing set
        processingConversations.value.delete(conversationId)

        // Get messages for this conversation
        const convMessages = conversationMessages.value.get(conversationId) || []
        const messageIndex = convMessages.findIndex((msg) => msg.id === messageId)

        if (messageIndex !== -1) {
          // Mark message as complete - create new array for reactivity
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
            conversationMessages.value.set(conversationId, updatedMessages)
          }
        }

        // Update current messages if this is the active conversation
        if (conversationId === currentConversationId.value) {
          messages.value = [...convMessages]
        }

        // Reset loading state when streaming is complete
        if (conversationId === currentConversationId.value) {
          isLoading.value = false
        }
      }

      console.log('DEBUG: Conversation processing completed:', conversationId)
    })

    // Conversation created
    webSocketService.onMessage('conversation_created', (data: any) => {
      if (data.conversation) {
        conversations.value.set(data.conversation.id, data.conversation)
        currentConversationId.value = data.conversation.id
        messages.value = []
        // Reset loading state after conversation is created
        isLoading.value = false
      }
    })

    // Conversations list
    webSocketService.onMessage('conversations_list', (data: any) => {
      console.log('DEBUG: Received conversations list:', data)
      if (data.conversations && Array.isArray(data.conversations)) {
        data.conversations.forEach((conv: Conversation) => {
          conversations.value.set(conv.id, conv)
        })
        console.log('DEBUG: Loaded conversations:', Array.from(conversations.value.keys()))
      } else {
        console.error('DEBUG: Invalid conversations data:', data)
      }
    })

    // Conversation details
    webSocketService.onMessage('conversation_details', (data: any) => {
      console.log('DEBUG: Received conversation details:', data)
      if (data.conversation) {
        // data.conversation contains both the conversation details and messages
        const conversationWithMessages = data.conversation
        const conversation = conversationWithMessages.conversation
        const convMessages = conversationWithMessages.messages

        conversations.value.set(conversation.id, conversation)
        console.log('DEBUG: Conversation messages loaded:', convMessages?.length || 0)

        if (convMessages && Array.isArray(convMessages)) {
          // Ensure each message has a unique id
          const safeMessages = convMessages.map((msg: any) => {
            if (!msg.id) {
              msg.id = `msg-${Date.now()}-${Math.random().toString(36).substr(2, 5)}`
            }
            return msg
          })
          messages.value = safeMessages
          console.log('DEBUG: Messages set in store:', messages.value.length)
        }

        currentConversationId.value = conversation.id
        // Reset loading state after conversation details are loaded
        isLoading.value = false
      } else {
        console.error('DEBUG: Invalid conversation details data:', data)
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
        const msg = data.message
        // Ensure message has a unique id
        if (!msg.id) {
          msg.id = `msg-${Date.now()}`
        }

        // Check if message already exists to avoid duplicates
        const existingIndex = messages.value.findIndex(
          (m) =>
            m.role === 'user' &&
            m.content === msg.content &&
            Math.abs(new Date(m.created_at).getTime() - new Date(msg.created_at).getTime()) < 5000,
        )

        if (existingIndex === -1) {
          // Only add if not already present
          messages.value.push(msg)
        }
      }
    })

    webSocketService.onMessage('chat_interrupted', (data: any) => {
      loadConversation()
    })

    // Tool execution started
    webSocketService.onMessage('tool_execution_started', (data: any) => {
      // Update tool call status to "executing" in existing messages
      updateToolCallStatus(data.tool_call_id, data.message_id, 'executing')
    })

    // Tool execution completed
    webSocketService.onMessage('tool_execution_completed', (data: any) => {
      // Update tool call status to "completed" and store result
      updateToolCallStatus(data.tool_call_id, data.conversation_id, 'completed', data.result)
    })

    // Tool execution failed
    webSocketService.onMessage('tool_execution_failed', (data: any) => {
      // Update tool call status to "failed" and store error
      updateToolCallStatus(data.tool_call_id, data.conversation_id, 'failed', data.error)
    })

    // Tool execution (legacy - used for broadcast tool status updates)
    webSocketService.onMessage('tool_execution', (data: any) => {
      // Update tool call status in existing messages
      if (data.message_id && data.tool_index !== undefined) {
        const messageIndex = messages.value.findIndex((msg) => msg.id === data.message_id)
        if (messageIndex !== -1 && messages.value[messageIndex]?.tool_calls) {
          messages.value[messageIndex].tool_calls![data.tool_index] = data.tool_call
        }
      }
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
    if (!content.trim() || !currentConversationId.value || isLoading.value) return

    // Add this conversation to processing set
    if (currentConversationId.value) {
      console.log('🎯 ADDING CONVERSATION TO PROCESSING SET:', currentConversationId.value)
      processingConversations.value.add(currentConversationId.value)
      console.log('🎯 PROCESSING CONVERSATIONS NOW:', Array.from(processingConversations.value))
    }

    // Create user message immediately for better UX
    const userMessage: ChatMessage = {
      id: `msg-${Date.now()}`,
      conversation_id: currentConversationId.value || '',
      role: 'user',
      content: content.trim(),
      created_at: new Date().toISOString(),
    }

    messages.value.push(userMessage)
    isLoading.value = true

    // Send to WebSocket with conversation ID
    webSocketService.sendMessageToAssistant(currentConversationId.value, content)
  }

  const createConversation = async (title?: string, initialMessage?: string) => {
    isLoading.value = true
    webSocketService.createConversation(title, initialMessage)
    // Response will be handled by message handlers
    // Note: User message will be handled by user_message_sent event from backend
  }

  const loadConversations = async () => {
    try {
      console.log('DEBUG: Requesting conversations from API')
      const response = await apiClient.getConversations()

      if (response.success && response.conversations) {
        console.log('DEBUG: API conversations response:', response.conversations)
        response.conversations.forEach((conv: ApiConversation) => {
          // Convert API conversation to WebSocket conversation format
          const wsConv: Conversation = {
            id: conv.id,
            title: conv.title,
            user_id: conv.user_id,
            project_id: conv.project_id,
            status: conv.status, // 🎯 NEW: Include status from API
            created_at: conv.created_at,
            updated_at: conv.updated_at,
          }
          console.log('DEBUG: Adding conversation to store:', wsConv)
          conversations.value.set(conv.id, wsConv)
        })
        console.log('DEBUG: Loaded conversations via API:', response.conversations.length)
        console.log(
          'DEBUG: Store conversations after loading:',
          Array.from(conversations.value.values()),
        )
      } else {
        console.error('DEBUG: Failed to load conversations via API:', response)
      }
    } catch (error) {
      console.error('DEBUG: Error loading conversations via API:', error)
    }
  }

  const loadConversation = async (conversationId: string) => {
    try {
      console.log('DEBUG: Loading conversation via API:', conversationId)
      isLoadingHistory.value = true

      const response = await apiClient.getConversationMessages(conversationId)
      console.log('🔍 LOAD CONVERSATION RESPONSE:', response)

      if (response.success && response.conversation) {
        const { conversation, messages: apiMessages } = response.conversation

        // Convert API conversation to WebSocket format
        const wsConv: Conversation = {
          id: conversation.id,
          title: conversation.title,
          user_id: conversation.user_id,
          status: conversation.status,
          project_id: conversation.project_id,
          created_at: conversation.created_at,
          updated_at: conversation.updated_at,
        }

        // Convert API messages to WebSocket format
        const wsMessages: ChatMessage[] = apiMessages.map((apiMsg: ApiMessage) => ({
          id: apiMsg.id,
          conversation_id: apiMsg.conversation_id,
          role: apiMsg.role as 'user' | 'assistant' | 'system',
          content: apiMsg.content,
          metadata: apiMsg.metadata || {},
          tool_calls:
            apiMsg.tool_calls?.map((toolCall) => ({
              id: toolCall.id,
              type: toolCall.type,
              function: {
                name: toolCall.function.name,
                arguments: toolCall.function.arguments,
              },
              status: toolCall.status,
              result: toolCall.result,
              error: toolCall.error,
            })) || [],
          created_at: apiMsg.created_at,
        }))

        // Update store - always store conversation and messages
        conversations.value.set(conversation.id, wsConv)
        conversationMessages.value.set(conversation.id, wsMessages)

        // Update UI messages if this is still the current conversation
        if (currentConversationId.value === conversationId) {
          messages.value = wsMessages
        }

        if (conversation.status === 'processing') {
          // TODO: get the new chunk data in backend memory, then continue appending the stream data
          // get_streaming_conversation -> to get current streaming conversation, check if id same with currentConversationId
          // streaming_conversation_loaded -> this handler sent from the backend to get chunk message from memory
        }
      } else {
        console.error('DEBUG: Failed to load conversation via API:', response)
      }
    } catch (error) {
      console.error('DEBUG: Error loading conversation via API:', error)
    } finally {
      isLoadingHistory.value = false
    }
  }

  const deleteConversation = (conversationId: string) => {
    webSocketService.deleteConversation(conversationId)
  }

  const selectConversation = (conversationId: string) => {
    // Allow switching even if other conversations are processing
    if (conversations.value.has(conversationId)) {
      currentConversationId.value = conversationId

      // Switch to messages for this conversation
      const convMessages = conversationMessages.value.get(conversationId) || []
      messages.value = [...convMessages]

      // Load conversation history if not already loaded
      if (!conversationMessages.value.has(conversationId)) {
        loadConversation(conversationId)
      }
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

  const updateToolCallStatus = (
    toolCallId: string,
    messageIdOrConvId: string,
    status: string,
    result?: any,
  ) => {
    // Find the message containing this tool call
    // Note: For execution events, backend might send conversation_id instead of message_id
    const messageIndex = messages.value.findIndex(
      (msg) =>
        msg.id === messageIdOrConvId || // Try matching by message ID first
        (msg.tool_calls && msg.tool_calls.some((tc) => tc.id === toolCallId)), // Or find message containing the tool call
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

  return {
    // State
    conversations,
    currentConversationId,
    currentConversation,
    conversationMessages,
    messages,
    isLoading,
    isConnected,
    connectionStatus,
    processingConversations,
    isLoadingHistory,

    // Computed
    messageCount,
    hasMessages,
    currentConversationMessages,
    isProcessing,
    canChat,
    anyConversationProcessing,

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
