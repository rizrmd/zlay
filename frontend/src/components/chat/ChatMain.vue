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
import { ref, nextTick } from 'vue'
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

defineProps<Props>()

const emit = defineEmits<{
  'send-message': [content: string]
}>()

const messagesContainer = ref<HTMLElement | null>(null)

const handleSendMessage = async (content: string) => {
  emit('send-message', content)
  
  // Scroll to bottom
  await nextTick()
  scrollToBottom()
}

const scrollToBottom = () => {
  if (messagesContainer.value) {
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  }
}

defineExpose({
  messagesContainer
})
</script>