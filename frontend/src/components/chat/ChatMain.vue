<template>
  <div
    :class="[
      'flex flex-col flex-1 transition-all duration-300',
      sidebarOpen && isMobile ? 'ml-80' : 'ml-0',
    ]"
  >
    <!-- Chat Messages -->
    <div ref="messagesContainer" class="flex-1 overflow-y-auto space-y-4 relative">
      <WelcomeMessage :has-messages="hasMessages" :is-connected="isConnected" />

      <MessageList :messages="messages" />

      <LoadingIndicator :is-loading="isLoading" />

      <!-- Scroll to bottom button - Fixed positioning for true floating -->
      <button
        v-if="showScrollToBottomButton"
        @click="scrollToBottom(true)"
        class="fixed z-50 bg-primary text-primary-foreground p-3 rounded-full shadow-lg hover:bg-primary/90 transition-all duration-200 hover:scale-105"
        title="Scroll to bottom"
        :style="{ bottom: '80px', right: '20px' }"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M19 14l-7 7m0 0l-7-7m7 7V3"
          ></path>
        </svg>
      </button>
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
import { ref, nextTick, watch, onMounted, onBeforeUnmount } from 'vue'
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
const lastStreamingContent = ref('')
const showScrollToBottomButton = ref(false)

const handleSendMessage = async (content: string) => {
  emit('send-message', content)

  // Smooth scroll to bottom after sending message
  await nextTick()
  scrollToBottom(true)
}

const scrollToBottom = (smooth = false) => {
  if (messagesContainer.value) {
    const scrollOptions: ScrollToOptions = {
      top: messagesContainer.value.scrollHeight,
    }

    // Use smooth scrolling for user interactions, instant for streaming
    if (smooth) {
      scrollOptions.behavior = 'smooth'
    }

    messagesContainer.value.scrollTo(scrollOptions)
    console.log('📜 Scrolled to bottom:', {
      smooth,
      scrollHeight: messagesContainer.value.scrollHeight,
    })
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

  const scrollHeight = messagesContainer.value.scrollHeight
  const scrollTop = messagesContainer.value.scrollTop
  const clientHeight = messagesContainer.value.clientHeight
  const distanceFromBottom = scrollHeight - scrollTop - clientHeight

  const isFirstLoad = previousMessageCount.value === 0
  const isNearBottom = distanceFromBottom < 150

  // Also always scroll during streaming (loading=true) or if container is empty
  const shouldAutoScroll =
    isFirstLoad || isNearBottom || props.isLoading || scrollHeight <= clientHeight

  // Show/hide scroll to bottom button based on scroll position
  showScrollToBottomButton.value = !isNearBottom && props.messages.length > 0

  if (shouldAutoScroll) {
    console.log('📜 Auto-scrolling triggered:', {
      isFirstLoad,
      isNearBottom,
      isLoading: props.isLoading,
      distanceFromBottom,
      scrollHeight,
      clientHeight,
      messageCount: props.messages.length,
      showScrollButton: showScrollToBottomButton.value,
    })
    // Use smooth scrolling for user interactions, instant for streaming
    scrollToBottom(props.isLoading)
  } else {
    console.log('📜 Skipping auto-scroll - user scrolled up', {
      distanceFromBottom,
      messageCount: props.messages.length,
      showScrollButton: showScrollToBottomButton.value,
    })
  }
}

// Watch for message count changes
watch(
  () => props.messages.length,
  async (newCount, oldCount) => {
    console.log('📝 Message count changed:', { from: oldCount, to: newCount })
    previousMessageCount.value = newCount
    await nextTick()
    handleAutoScroll()
  },
)

// Watch for messages array reference changes (conversation load/switch)
watch(
  () => props.messages,
  async (newMessages, oldMessages) => {
    // Only trigger if it's a completely new array (conversation switch)
    if (newMessages !== oldMessages) {
      console.log('🔄 Messages array reference changed (conversation loaded/switched)', {
        newCount: newMessages?.length || 0,
        oldCount: oldMessages?.length || 0,
      })
      await nextTick()
      scrollToBottom(true)
      handleAutoScroll()
    }
  },
  { flush: 'post' },
)

// 🔥 REAL-TIME DEBUG: Watch for actual message content changes
watch(
  () => props.messages,
  async (newMessages, oldMessages) => {
    if (newMessages && newMessages.length > 0) {
      const lastMessage = newMessages[newMessages.length - 1]

      // Check if this is a streaming assistant message with new content
      if (lastMessage?.role === 'assistant' && lastMessage?.content) {
        const currentContent = lastMessage.content
        const contentChanged = currentContent !== lastStreamingContent.value

        if (contentChanged) {
          lastStreamingContent.value = currentContent
          console.log('🌊 Streaming content detected:', {
            contentLength: currentContent.length,
            contentPreview:
              currentContent.substring(0, 50) + (currentContent.length > 50 ? '...' : ''),
          })

          await nextTick()
          handleAutoScroll()
        }
      }

      console.log('🔍 Real-time message update:', {
        messageCount: newMessages.length,
        lastMessageId: lastMessage?.id,
        lastMessageRole: lastMessage?.role,
        lastContentLength: lastMessage?.content?.length || 0,
        timestamp: new Date().toISOString(),
      })
    }
  },
  { deep: true },
)

// Watch for loading state changes (streaming)
watch(
  () => props.isLoading,
  async (newLoading, oldLoading) => {
    if (newLoading !== previousLoadingState.value) {
      console.log('⏳ Loading state changed:', { from: oldLoading, to: newLoading })
      previousLoadingState.value = newLoading
      await handleAutoScroll()
    }
  },
)

// Auto-scroll when component mounts (conversation opened)
onMounted(async () => {
  console.log('🚀 ChatMain mounted, initial auto-scroll')

  // Multiple attempts to ensure scroll works
  const scrollToBottomOnMount = async () => {
    await nextTick()
    await handleAutoScroll()

    // Double-check after a small delay for content rendering
    setTimeout(async () => {
      await nextTick()
      await handleAutoScroll()
    }, 100)
  }

  await scrollToBottomOnMount()

  // Add scroll listener for showing/hiding scroll-to-bottom button
  if (messagesContainer.value) {
    messagesContainer.value.addEventListener('scroll', handleScroll)
  }
})

// Handle scroll events to show/hide scroll button
const handleScroll = () => {
  if (!messagesContainer.value) return

  const scrollHeight = messagesContainer.value.scrollHeight
  const scrollTop = messagesContainer.value.scrollTop
  const clientHeight = messagesContainer.value.clientHeight
  const distanceFromBottom = scrollHeight - scrollTop - clientHeight

  showScrollToBottomButton.value = distanceFromBottom > 150 && props.messages.length > 0
}

// Cleanup scroll listener
onBeforeUnmount(() => {
  if (messagesContainer.value) {
    messagesContainer.value.removeEventListener('scroll', handleScroll)
  }
})

defineExpose({
  messagesContainer,
})
</script>
