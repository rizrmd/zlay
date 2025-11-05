import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Conversation } from '@/services/websocket'
import { apiClient } from '@/services/api'
import webSocketService from '@/services/websocket'

export const useConversationStore = defineStore('conversation', () => {
  // State
  const conversations = ref<Map<string, Conversation>>(new Map())
  const currentConversationId = ref<string | null>(null)

  // Store messages per conversation for independent streaming
  const conversationMessages = ref<Map<string, any[]>>(new Map())

  // Track which conversations are currently being processed (multiple can be processing)
  const processingConversations = ref<Set<string>>(new Set())

  // Loading states
  const isLoading = ref(false)
  const isLoadingHistory = ref(false)
  const isLoadingConversations = ref(false)

  // Computed
  const currentConversation = computed(() => {
    if (!currentConversationId.value) return null
    return conversations.value.get(currentConversationId.value) || null
  })

  const currentConversationMessages = computed(() => {
    if (!currentConversationId.value) return []
    return conversationMessages.value.get(currentConversationId.value) || []
  })

  const messageCount = computed(() => currentConversationMessages.value.length)
  const hasMessages = computed(() => messageCount.value > 0)

  // ✅ IMPROVED: Check if current conversation is processing (from conversation status)
  const isCurrentConversationProcessing = computed(() => {
    if (!currentConversationId.value) return false

    const currentConv = conversations.value.get(currentConversationId.value)
    const isStatusProcessing = currentConv?.status === 'processing'
    const isInProcessingSet = processingConversations.value.has(currentConversationId.value)

    console.log('💬 PROCESSING CHECK:', {
      conversationId: currentConversationId.value,
      conversationStatus: currentConv?.status,
      isStatusProcessing,
      isInProcessingSet,
      finalResult: isStatusProcessing || isInProcessingSet,
    })

    return isStatusProcessing || isInProcessingSet
  })

  const isProcessing = computed(() => isLoading.value || isLoadingHistory.value)
  const anyConversationProcessing = computed(() => processingConversations.value.size > 0)

  // Legacy messages for UI compatibility
  const messages = ref<any[]>([])

  // Actions
  const selectConversation = (conversationId: string) => {
    if (conversations.value.has(conversationId)) {
      console.log('🔄 SELECTING CONVERSATION:', {
        fromConversationId: currentConversationId.value,
        toConversationId: conversationId,
        fromMessageCount: currentConversationId.value
          ? conversationMessages.value.get(currentConversationId.value)?.length || 0
          : 0,
        toMessageCount: conversationMessages.value.get(conversationId)?.length || 0,
      })

      currentConversationId.value = conversationId

      // Switch to messages for this conversation
      const convMessages = conversationMessages.value.get(conversationId) || []
      messages.value = [...convMessages]

      console.log('✅ CONVERSATION SWITCHED:', {
        newConversationId: currentConversationId.value,
        newMessageCount: messages.value.length,
        messages: messages.value.map((m) => ({
          id: m.id,
          role: m.role,
          contentPreview: `"${m.content?.substring(0, 30)}${m.content?.length > 30 ? '...' : ''}"`,
          length: m.content?.length || 0,
        })),
      })
    } else {
      console.warn('⚠️ Cannot select conversation - not found:', conversationId)
    }
  }

  const clearCurrentConversation = () => {
    currentConversationId.value = null
    messages.value = []
  }

  const loadConversations = async () => {
    // Prevent duplicate loading
    if (isLoadingConversations.value) {
      console.log('🔄 loadConversations: Already loading, skipping...')
      return
    }

    try {
      console.log('🚀 loadConversations: Starting API call...', {
        currentSize: conversations.value.size,
        isLoadingConversations: isLoadingConversations.value
      })
      isLoadingConversations.value = true
      const response = await apiClient.getConversations()

      if (response.success && response.conversations) {
        response.conversations.forEach((conv: any) => {
          const wsConv: Conversation = {
            id: conv.id,
            title: conv.title,
            user_id: conv.user_id,
            status: conv.status,
            project_id: conv.project_id,
            created_at: conv.created_at,
            updated_at: conv.updated_at,
          }
          conversations.value.set(conv.id, wsConv)
        })
        console.log('DEBUG: Loaded conversations:', response.conversations.length)
      }
    } catch (error) {
      console.error('DEBUG: Error loading conversations:', error)
    } finally {
      isLoadingConversations.value = false
    }
  }

  const loadConversation = async (conversationId: string) => {
    try {
      console.log('🔍 LOAD CONVERSATION STARTED:', conversationId)
      console.log('🔍 ALL CONVERSATIONS IN STORE:', conversations.value)
      console.log('🔍 CONVERSATION BY ID:', conversations.value.get(conversationId))
      console.log('🔍 ALL CONVERSATION IDS:', Array.from(conversations.value.keys()))
      console.log(
        '🔍 SEARCHING FOR ID:',
        conversationId,
        'FOUND:',
        conversations.value.has(conversationId),
      )

      // ✅ NEW: Check if conversation is currently processing before reloading
      const existingConversation = conversations.value.get(conversationId)
      if (existingConversation?.status === 'processing') {
        console.log(
          '🔄 SKIPPING LOAD: Conversation is currently processing, preserving streaming state:',
          conversationId,
        )
        return
      }

      isLoadingHistory.value = true

      const response = await apiClient.getConversationMessages(conversationId)

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
        const wsMessages = apiMessages.map((apiMsg: any) => ({
          id: apiMsg.id,
          conversation_id: apiMsg.conversation_id,
          role: apiMsg.role,
          content: apiMsg.content,
          metadata: apiMsg.metadata || {},
          tool_calls:
            apiMsg.tool_calls?.map((toolCall: any) => ({
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

        // Update store
        conversations.value.set(conversation.id, wsConv)
        conversationMessages.value.set(conversation.id, wsMessages)

        // Update UI messages if this is still the current conversation
        if (currentConversationId.value === conversationId) {
          console.log('💬 CONVERSATION STORE: Updating UI messages, count:', wsMessages.length)
          console.log('💬 CONVERSATION STORE: Sample message:', wsMessages[0])
          messages.value = wsMessages
          console.log('💬 CONVERSATION STORE: Messages after update:', messages.value.length)
        } else {
          console.log(
            '💬 CONVERSATION STORE: Not updating UI, currentId:',
            currentConversationId.value,
            'loadId:',
            conversationId,
          )
        }
      }
    } catch (error) {
      console.error('DEBUG: Error loading conversation:', error)
    } finally {
      isLoadingHistory.value = false
    }
  }

  const createConversation = async (title?: string, initialMessage?: string) => {
    isLoading.value = true
    webSocketService.createConversation(title, initialMessage)
  }

  const deleteConversation = (conversationId: string) => {
    webSocketService.deleteConversation(conversationId)
  }

  // Streaming state management
  const addToProcessing = (conversationId: string) => {
    processingConversations.value.add(conversationId)
    console.log('🎯 ADDED TO PROCESSING:', conversationId)
    console.log('🎯 PROCESSING CONVERSATIONS NOW:', Array.from(processingConversations.value))
  }

  const removeFromProcessing = (conversationId: string) => {
    processingConversations.value.delete(conversationId)
    console.log('🎯 REMOVED FROM PROCESSING:', conversationId)
  }

  const isConversationProcessing = (conversationId: string) => {
    return processingConversations.value.has(conversationId)
  }

  // Message handling for streaming
  const addOrUpdateMessage = (conversationId: string, message: any) => {
    // Ensure conversationMessages is initialized
    if (!conversationMessages.value) {
      console.error('❌ conversationMessages is undefined in addOrUpdateMessage')
      return
    }

    // 🔍 DEBUG: Log incoming message details
    // console.log('🔍 ADD/UPDATE MESSAGE CALLED:', {
    //   conversationId,
    //   messageId: message.id,
    //   messageContent: message.content,
    //   contentLength: message.content?.length || 0,
    //   isCurrentConversation: currentConversationId.value === conversationId,
    //   existingMessagesCount: conversationMessages.value.get(conversationId)?.length || 0,
    // })

    const convMessages = conversationMessages.value.get(conversationId) || []
    const existingIndex = convMessages.findIndex((msg) => msg.id === message.id)

    if (existingIndex !== -1) {
      // Update existing message (for streaming) - REPLACE content since backend sends accumulated
      const existingMessage = convMessages[existingIndex]

      // console.log('📝 EXISTING MESSAGE FOUND:', {
      //   messageId: message.id,
      //   existingContent: `"${existingMessage.content}"`,
      //   existingLength: existingMessage.content?.length || 0,
      //   newContent: `"${message.content}"`,
      //   newLength: message.content?.length || 0,
      //   willReplace: !!message.content,
      // })

      const updatedMessage = {
        ...existingMessage,
        // Only update fields that are provided
        ...(message.content && {
          content: message.content, // 🔄 REPLACE content since backend sends accumulated
        }),
        ...(message.metadata && { metadata: { ...existingMessage.metadata, ...message.metadata } }),
        ...(message.tool_calls && { tool_calls: message.tool_calls }),
        ...(message.created_at && { created_at: message.created_at }),
      }

      // console.log('💬 CONVERSATION STORE: Streaming replacement:', {
      //   messageId: message.id,
      //   oldContent: `"${existingMessage.content}"`,
      //   newContent: `"${message.content}"`,
      //   replacedContent: `"${updatedMessage.content}"`,
      //   contentLength: updatedMessage.content.length,
      // })

      const updatedMessages = [...convMessages]
      updatedMessages[existingIndex] = updatedMessage
      conversationMessages.value.set(conversationId, updatedMessages)
    } else {
      // Add new message
      // console.log(
      //   '💬 CONVERSATION STORE: Adding new message:',
      //   message.id,
      //   `"${message.content?.substring(0, 50)}${message.content?.length > 50 ? '...' : ''}"`,
      // )
      conversationMessages.value.set(conversationId, [...convMessages, message])
    }

    // Update UI if this is current conversation
    if (currentConversationId.value === conversationId) {
      if (conversationMessages.value) {
        const finalMessages = conversationMessages.value.get(conversationId) || []
        messages.value = finalMessages
        console.log('💬 CONVERSATION STORE: UI messages updated, count:', finalMessages.length)
        console.log('💬 FINAL UI MESSAGES (REPLACE MODE):', finalMessages.map((m) => ({
          id: m.id,
          role: m.role,
          content: `"${m.content?.substring(0, 30)}${m.content?.length > 30 ? '...' : ''}"`,
          length: m.content?.length || 0,
          replaceMode: true,
        })))
      } else {
        console.error('❌ conversationMessages is undefined when updating UI')
      }
    } else {
      console.log('💬 SKIPPING UI UPDATE - not current conversation:', {
        targetConversationId: conversationId,
        currentConversationId: currentConversationId.value,
      })
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
    isLoadingHistory,
    isLoadingConversations, // ✅ NEW: Prevent duplicate conversation loading
    processingConversations,

    // Computed
    messageCount,
    hasMessages,
    currentConversationMessages,
    isProcessing,
    anyConversationProcessing,
    isCurrentConversationProcessing, // ✅ NEW: Individual conversation processing state

    // Actions
    selectConversation,
    clearCurrentConversation,
    loadConversations,
    loadConversation,
    createConversation,
    deleteConversation,
    addToProcessing,
    removeFromProcessing,
    isConversationProcessing,
    addOrUpdateMessage,
  }
})
