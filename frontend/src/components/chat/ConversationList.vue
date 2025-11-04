<template>
  <div class="flex-1 overflow-y-auto p-4 space-y-2">
    <div v-if="conversations.length === 0" class="text-center text-muted-foreground py-8">
      <svg class="w-8 h-8 mx-auto mb-2 opacity-50" xmlns="http://www.w3.org/2000/svg" fill="none"
        viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
          d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
      </svg>
      <p class="text-sm">No conversations yet</p>
      <p class="text-xs mt-1">Click "New chat" to start one</p>
    </div>

    <Card
      v-for="conversation in conversations"
      :key="conversation.id"
      class="cursor-pointer hover:bg-accent transition-colors"
      :class="{ 'ring-2 ring-primary': currentConversationId === conversation.id }"
      @click="navigateToConversation(conversation.id)"
    >
      <CardContent class="flex items-center justify-between p-4">
        <div class="flex-1 min-w-0">
          <div class="font-medium text-sm truncate flex items-center gap-2">
            {{ conversation.title }}
            <div 
              v-if="isConversationProcessing(conversation.id)" 
              class="flex items-center gap-1"
              title="Assistant is responding"
            >
              <div class="w-1.5 h-1.5 bg-green-500 rounded-full animate-pulse"></div>
              <span class="text-xs text-green-600 font-medium">Live</span>
            </div>
            <div 
              v-else-if="isConversationLoading(conversation.id)" 
              class="flex items-center gap-1"
              title="Loading conversation..."
            >
              <div class="w-1.5 h-1.5 bg-blue-500 rounded-full animate-pulse"></div>
              <span class="text-xs text-blue-600 font-medium">Loading</span>
            </div>
          </div>
          <div class="text-xs text-muted-foreground mt-1">
            {{ formatMessageTime(conversation) }}
          </div>
        </div>
        <Button variant="ghost" size="icon" @click.stop="$emit('delete-conversation', conversation.id)">
          <svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"
            stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
              d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        </Button>
      </CardContent>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import type { Conversation } from '@/services/websocket'
import { useChatStore } from '@/stores/chat'
import { useRouter, useRoute } from 'vue-router'

interface Props {
  conversations: Conversation[]
  currentConversationId: string | null
}

defineProps<Props>()

const emit = defineEmits<{
  'select-conversation': [id: string]
  'delete-conversation': [id: string]
}>()

const route = useRoute()
const router = useRouter()
const chatStore = useChatStore()

// Check if a specific conversation is currently processing
const isConversationProcessing = (conversationId: string): boolean => {
  const conversation = chatStore.conversations.get(conversationId)
  return conversation?.status === 'processing' || false
}

// Check if a specific conversation is currently loading
const isConversationLoading = (conversationId: string): boolean => {
  const conversation = chatStore.conversations.get(conversationId)
  return chatStore.isLoadingHistory || false
}

const formatMessageTime = (conversation: Conversation) => {
  return chatStore.formatMessageTime(conversation as any)
}

const navigateToConversation = (conversationId: string) => {
  // Navigate to conversation-specific URL for better refresh support
  const projectId = route.params.id as string
  router.push(`/p/${projectId}/chat/${conversationId}`)
}
</script>