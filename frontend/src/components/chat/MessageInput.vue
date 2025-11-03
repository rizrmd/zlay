<template>
  <div class="border-t p-4">
    <div class="max-w-4xl mx-auto">
      <div class="flex items-end space-x-4">
        <div class="flex-1">
          <Textarea
            v-model="messageContent"
            @keydown="handleKeyDown"
            @input="autoResize"
            ref="messageInputRef"
            placeholder="Type your message..."
            :rows="1"
            :disabled="isLoading || !isConnected"
            class="resize-none"
          />
        </div>
        <Button @click="sendMessage" :disabled="!messageContent.trim() || isLoading || !isConnected">
          <svg v-if="!isLoading" class="w-4 h-4 mr-2" xmlns="http://www.w3.org/2000/svg" fill="none"
            viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
              d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
          </svg>
          <svg v-else class="w-4 h-4 mr-2 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none"
            viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z">
            </path>
          </svg>
          {{ isLoading ? 'Sending...' : 'Send' }}
        </Button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick } from 'vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

interface Props {
  isLoading: boolean
  isConnected: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  'send-message': [content: string]
}>()

const messageContent = ref('')
const messageInputRef = ref<HTMLTextAreaElement | null>(null)

const sendMessage = async () => {
  if (!messageContent.value.trim()) return

  const content = messageContent.value.trim()
  messageContent.value = ''
  
  emit('send-message', content)

  // Scroll to bottom
  await nextTick()
  scrollToBottom()
}

const handleKeyDown = (event: KeyboardEvent) => {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    sendMessage()
  }
}

const autoResize = () => {
  if (messageInputRef.value) {
    messageInputRef.value.style.height = 'auto'
    messageInputRef.value.style.height = Math.min(messageInputRef.value.scrollHeight, 120) + 'px'
  }
}

const scrollToBottom = () => {
  // This will be handled by the parent component
}

defineExpose({
  messageInputRef
})
</script>