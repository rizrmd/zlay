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
  const isProcessing = computed(() => isLoading.value || isLoadingHistory.value)
  const anyConversationProcessing = computed(() => processingConversations.value.size > 0)
  
  // Legacy messages for UI compatibility
  const messages = ref<any[]>([])
  
  // Actions
  const selectConversation = (conversationId: string) => {
    if (conversations.value.has(conversationId)) {
      currentConversationId.value = conversationId
      
      // Switch to messages for this conversation
      const convMessages = conversationMessages.value.get(conversationId) || []
      messages.value = [...convMessages]
    }
  }
  
  const clearCurrentConversation = () => {
    currentConversationId.value = null
    messages.value = []
  }
  
  const loadConversations = async () => {
    try {
      console.log('DEBUG: Loading conversations from API')
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
    }
  }
  
  const loadConversation = async (conversationId: string) => {
    try {
      console.log('🔍 LOAD CONVERSATION STARTED:', conversationId)
      console.log('🔍 ALL CONVERSATIONS IN STORE:', conversations.value)
      console.log('🔍 CONVERSATION BY ID:', conversations.value.get(conversationId))
      console.log('🔍 ALL CONVERSATION IDS:', Array.from(conversations.value.keys()))
      console.log('🔍 SEARCHING FOR ID:', conversationId, 'FOUND:', conversations.value.has(conversationId))
      
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
          tool_calls: apiMsg.tool_calls?.map((toolCall: any) => ({
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
          console.log('💬 CONVERSATION STORE: Not updating UI, currentId:', currentConversationId.value, 'loadId:', conversationId)
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
    const convMessages = conversationMessages.value.get(conversationId) || []
    const existingIndex = convMessages.findIndex((msg) => msg.id === message.id)
    
    if (existingIndex !== -1) {
      // Update existing message (for streaming) - ACCUMULATE content!
      const existingMessage = convMessages[existingIndex]
      const updatedMessage = {
        ...existingMessage,
        // Only update fields that are provided
        ...(message.content && { 
          content: existingMessage.content + message.content  // ✅ ACCUMULATE content
        }),
        ...(message.metadata && { metadata: { ...existingMessage.metadata, ...message.metadata } }),
        ...(message.tool_calls && { tool_calls: message.tool_calls }),
        ...(message.created_at && { created_at: message.created_at }),
      }
      
      console.log('💬 CONVERSATION STORE: Streaming update:', {
        messageId: message.id,
        oldContent: existingMessage.content,
        newContent: message.content,
        combinedContent: updatedMessage.content,
        contentLength: updatedMessage.content.length
      })
      
      const updatedMessages = [...convMessages]
      updatedMessages[existingIndex] = updatedMessage
      conversationMessages.value.set(conversationId, updatedMessages)
    } else {
      // Add new message
      console.log('💬 CONVERSATION STORE: Adding new message:', message.id, message.content?.substring(0, 50) + '...')
      conversationMessages.value.set(conversationId, [...convMessages, message])
    }
    
    // Update UI if this is current conversation
    if (currentConversationId.value === conversationId) {
      messages.value = conversationMessages.value.get(conversationId) || []
      console.log('💬 CONVERSATION STORE: UI messages updated, count:', messages.value.length)
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
    processingConversations,
    
    // Computed
    messageCount,
    hasMessages,
    currentConversationMessages,
    isProcessing,
    anyConversationProcessing,
    
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