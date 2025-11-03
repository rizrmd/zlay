import { ref } from 'vue'
import { useChatStore } from '@/stores/chat'

export const useChatActions = () => {
  const chatStore = useChatStore()
  const currentMessage = ref('')

  const sendMessage = async (content?: string) => {
    const messageContent = content || currentMessage.value.trim()
    if (!messageContent) return

    if (!content) {
      currentMessage.value = ''
    }

    if (chatStore.currentConversationId) {
      await chatStore.sendMessage(messageContent)
    } else {
      // Create new conversation with initial message
      await chatStore.createConversation(undefined, messageContent)
    }
  }

  const createNewConversation = async (initialMessage?: string) => {
    await chatStore.createConversation(undefined, initialMessage)
    if (!initialMessage) {
      currentMessage.value = ''
    }
  }

  const selectConversation = (conversationId: string) => {
    chatStore.selectConversation(conversationId)
  }

  const deleteConversation = async (conversationId: string) => {
    if (confirm('Are you sure you want to delete this conversation?')) {
      await chatStore.deleteConversation(conversationId)
    }
  }

  return {
    currentMessage,
    sendMessage,
    createNewConversation,
    selectConversation,
    deleteConversation
  }
}