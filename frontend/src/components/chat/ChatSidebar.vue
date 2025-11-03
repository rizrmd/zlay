<template>
  <div ref="sidebar" :class="[
    'bg-card border-r flex flex-col fixed md:relative h-full z-40 transition-transform duration-300',
    sidebarOpen ? 'translate-x-0' : '-translate-x-full',
    'w-80'
  ]">
    <ChatSidebarHeader
      :is-connected="isConnected"
      @navigate-dashboard="$emit('navigate-dashboard')"
      @create-conversation="$emit('create-conversation')"
    />
    
    <ConversationList
      :conversations="conversations"
      :current-conversation-id="currentConversationId"
      @select-conversation="$emit('select-conversation', $event)"
      @delete-conversation="$emit('delete-conversation', $event)"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import ChatSidebarHeader from './ChatSidebarHeader.vue'
import ConversationList from './ConversationList.vue'
import type { Conversation } from '@/services/websocket'

interface Props {
  sidebarOpen: boolean
  isConnected: boolean
  conversations: Conversation[]
  currentConversationId: string | null
}

defineProps<Props>()

const emit = defineEmits<{
  'navigate-dashboard': []
  'create-conversation': []
  'select-conversation': [id: string]
  'delete-conversation': [id: string]
}>()

const sidebar = ref<HTMLElement | null>(null)

defineExpose({
  sidebar
})
</script>