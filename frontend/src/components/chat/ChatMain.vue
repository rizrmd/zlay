<template>
  <div :class="[
    'flex flex-col flex-1 transition-all duration-300',
    sidebarOpen && isMobile ? 'ml-80' : 'ml-0'
  ]">
    <!-- Chat Messages -->
    <div ref="messagesContainer" class="flex-1 overflow-y-auto space-y-4">
      <WelcomeMessage
        :has-messages="hasMessages"
        :is-connected="isConnected"
      />
      
      <MessageList :messages="messages" />
      
      <LoadingIndicator :is-loading="isLoading" />
    </div>

    <!-- Chat Input -->
    <MessageInput
      :is-loading="isLoading"
      :is-connected="isConnected"
      @send-message="handleSendMessage"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, watch, onMounted } from 'vue'
import WelcomeMessage from './WelcomeMessage.vue'
import MessageList from './MessageList.vue'
import LoadingIndicator from './LoadingIndicator.vue'
import MessageInput from './MessageInput.vue'
import type { ChatMessage } from '@/services/websocket'

interface Props {
  sidebarOpen: boolean
  isMobile: boolean
  isConnected: boolean
  hasMessages: boolean
  messages: ChatMessage[]
  isLoading: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'send-message': [content: string]
}>()

const messagesContainer = ref<HTMLElement | null>(null)
const previousMessageCount = ref(0)
const previousLoadingState = ref(false)

const handleSendMessage = async (content: string) => {
  emit('send-message', content)
  
  // Smooth scroll to bottom after sending message
  await nextTick()
  scrollToBottom(true)
}

const scrollToBottom = (smooth = false) => {
  if (messagesContainer.value) {
    const scrollOptions: ScrollToOptions = {
      top: messagesContainer.value.scrollHeight
    }
    
    // Use smooth scrolling for user interactions, instant for streaming
    if (smooth) {
      scrollOptions.behavior = 'smooth'
    }
    
    messagesContainer.value.scrollTo(scrollOptions)
  }
}

// 🚀 ENHANCED: Auto-scroll on message changes
const handleAutoScroll = async () => {
  await nextTick()
  
  if (!messagesContainer.value) return
  
  // Auto-scroll when:
  // 1. New messages arrive
  // 2. Loading state changes (streaming starts/stops)
  // 3. Conversation first loads
  
  // Check if user is near bottom (within 150px) or if it's first load
  const scrollHeight = messagesContainer.value.scrollHeight
  const scrollTop = messagesContainer.value.scrollTop
  const clientHeight = messagesContainer.value.clientHeight
  const distanceFromBottom = scrollHeight - scrollTop - clientHeight
  
  const isFirstLoad = previousMessageCount.value === 0
  const isNearBottom = distanceFromBottom < 150
  
  // Also always scroll during streaming (loading=true)
  const shouldAutoScroll = isFirstLoad || isNearBottom || props.isLoading
  
  if (shouldAutoScroll) {
    console.log('DEBUG: Auto-scrolling - isFirstLoad:', isFirstLoad, 'isNearBottom:', isNearBottom, 'isLoading:', props.isLoading, 'distanceFromBottom:', distanceFromBottom)
    // Use smooth scrolling for user interactions, instant for streaming
    scrollToBottom(props.isLoading)
  } else {
    console.log('DEBUG: Not auto-scrolling - user scrolled up, distanceFromBottom:', distanceFromBottom)
  }
}

// Watch for message count changes
watch(() => props.messages.length, async (newCount, oldCount) => {
  if (newCount !== previousMessageCount.value) {
    console.log('DEBUG: Message count changed from', oldCount, 'to', newCount, 'auto-scrolling')
    previousMessageCount.value = newCount
    await handleAutoScroll()
  }
})

// 🔥 REAL-TIME DEBUG: Watch for actual message content changes
watch(() => props.messages, async (newMessages, oldMessages) => {
  if (newMessages && newMessages.length > 0) {
    const lastMessage = newMessages[newMessages.length - 1]
    console.log('🔍 REAL-TIME MESSAGE UPDATE:', {
      messageCount: newMessages.length,
      lastMessageId: lastMessage?.id,
      lastMessageRole: lastMessage?.role,
      lastMessageContent: lastMessage?.content ? `"${lastMessage.content.substring(0, 50)}..."` : 'empty',
      lastContentLength: lastMessage?.content?.length || 0,
      timestamp: new Date().toISOString()
    })
  }
}, { deep: true })

// Watch for loading state changes (streaming)
watch(() => props.isLoading, async (newLoading, oldLoading) => {
  if (newLoading !== previousLoadingState.value) {
    console.log('DEBUG: Loading state changed from', oldLoading, 'to', newLoading, 'auto-scrolling')
    previousLoadingState.value = newLoading
    await handleAutoScroll()
  }
})

// Auto-scroll when component mounts (conversation opened)
onMounted(async () => {
  console.log('DEBUG: ChatMain mounted, auto-scrolling to bottom')
  await handleAutoScroll()
})

defineExpose({
  messagesContainer
})
</script>