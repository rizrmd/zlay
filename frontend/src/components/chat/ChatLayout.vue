<template>
  <div class="flex h-screen bg-background">
    <ConnectionStatus :is-connected="isConnected" />

    <MobileMenuToggle :is-mobile="isMobile" :sidebar-open="sidebarOpen" @toggle-sidebar="toggleSidebar" />

    <ChatSidebar ref="sidebarRef" :sidebar-open="sidebarOpen" :is-connected="isConnected" :conversations="conversations"
      :current-conversation-id="currentConversationId" @navigate-dashboard="navigateToDashboard"
      @create-conversation="createNewConversation" @select-conversation="selectConversation"
      @delete-conversation="deleteConversation" />

    <ChatMain ref="chatMainRef" :sidebar-open="sidebarOpen" :is-mobile="isMobile" :is-connected="isConnected"
      :has-messages="hasMessages" :messages="messages" :is-loading="isLoading" @send-message="sendMessage" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useChatStore } from '@/stores/chat'
import { useAuthStore } from '@/stores/auth'
import ConnectionStatus from '@/components/chat/ConnectionStatus.vue'
import MobileMenuToggle from '@/components/chat/MobileMenuToggle.vue'
import ChatSidebar from '@/components/chat/ChatSidebar.vue'
import ChatMain from '@/components/chat/ChatMain.vue'
import type { Conversation } from '@/services/websocket'

// Stores
const chatStore = useChatStore()
const authStore = useAuthStore()

// Route
const route = useRoute()
const router = useRouter()

// Reactive state
const sidebarOpen = ref(true)
const isMobile = ref(false)

// Computed properties from chat store
const isConnected = computed(() => chatStore.isConnected)
const conversations = computed(() => Array.from(chatStore.conversations.values()))
const currentConversationId = computed(() => chatStore.currentConversationId)
const messages = computed(() => chatStore.messages)
const isLoading = computed(() => chatStore.isLoading)
const hasMessages = computed(() => messages.value.length > 0)

// Refs for components
const sidebarRef = ref()
const chatMainRef = ref()

// Methods
const toggleSidebar = () => {
  sidebarOpen.value = !sidebarOpen.value
}

const checkMobile = () => {
  const wasMobile = isMobile.value
  isMobile.value = window.innerWidth < 768

  if (wasMobile && !isMobile.value) {
    sidebarOpen.value = true
  } else if (isMobile.value) {
    sidebarOpen.value = false
  }
}

const sendMessage = async (content: string) => {
  if (currentConversationId.value) {
    await chatStore.sendMessage(content)
  } else {
    // Create new conversation with initial message
    await chatStore.createConversation(undefined, content)
  }
}

const createNewConversation = async () => {
  await chatStore.createConversation()
}

const selectConversation = (conversationId: string) => {
  chatStore.selectConversation(conversationId)
}

const deleteConversation = async (conversationId: string) => {
  if (confirm('Are you sure you want to delete this conversation?')) {
    await chatStore.deleteConversation(conversationId)
  }
}

const navigateToDashboard = () => {
  router.push('/dashboard')
}

const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (isMobile.value &&
    sidebarOpen.value &&
    sidebarRef.value?.sidebar &&
    !sidebarRef.value.sidebar.contains(target) &&
    !target.closest('.mobile-menu-toggle') &&
    !target.closest('.settings-dropdown')) {
    sidebarOpen.value = false
  }
}

// Watch for connection changes to load conversations
watch(isConnected, (connected) => {
  if (connected) {
    chatStore.loadConversations()
  }
})

// Lifecycle
onMounted(async () => {
  console.log('🔥 ChatLayout mounted!')

  checkMobile()
  window.addEventListener('resize', checkMobile)
  document.addEventListener('click', handleClickOutside)

  // Initialize project from route params
  const projectId = route.params.id as string
  console.log('Current project ID:', projectId)

  // Initialize WebSocket connection
  console.log('All cookies:', document.cookie)

  if (projectId) {
    try {
      await chatStore.initWebSocket(projectId)
    } catch (error) {
      console.error('Failed to initialize WebSocket:', error)
    }
  } else {
    console.log('Missing projectId:', { projectId: !!projectId })
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
  document.removeEventListener('click', handleClickOutside)
  // Clean up WebSocket connection
  chatStore.disconnectWebSocket()
})
</script>

<style scoped>
.rotate-90 {
  transform: rotate(90deg);
}

@media (min-width: 768px) {
  .translate-x-full {
    transform: translateX(0);
  }
}

.mobile-menu-toggle {
  pointer-events: auto;
}

/* Loading dots animation */
@keyframes bounce {

  0%,
  80%,
  100% {
    transform: scale(0);
  }

  40% {
    transform: scale(1);
  }
}

.animate-bounce {
  animation: bounce 1.4s infinite ease-in-out both;
}

.animate-bounce:nth-child(1) {
  animation-delay: -0.32s;
}

.animate-bounce:nth-child(2) {
  animation-delay: -0.16s;
}

/* Textarea auto-resize */
:deep(.resize-none) {
  resize: none;
  min-height: 44px;
  max-height: 120px;
}

/* Ensure proper vertical centering in cards */
:deep(.card-content) {
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
}
</style>