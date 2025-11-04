<template>
  <div class="flex h-screen bg-background">
    <ConnectionStatus :is-connected="isConnected" />

    <MobileMenuToggle
      :is-mobile="isMobile"
      :sidebar-open="sidebarOpen"
      @toggle-sidebar="toggleSidebar"
    />

    <ChatSidebar
      ref="sidebarRef"
      :sidebar-open="sidebarOpen"
      :is-connected="isConnected"
      :conversations="conversations"
      :current-conversation-id="currentConversationId"
      @navigate-dashboard="navigateToDashboard"
      @create-conversation="createNewConversation"
      @select-conversation="selectConversation"
      @delete-conversation="deleteConversation"
    />

    <ChatMain
      ref="chatMainRef"
      :sidebar-open="sidebarOpen"
      :is-mobile="isMobile"
      :is-connected="isConnected"
      :has-messages="hasMessages"
      :messages="messages"
      :is-loading="isLoading"
      @send-message="sendMessage"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch, nextTick, onBeforeUnmount } from 'vue'
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
const conversations = computed(() => {
  // Transform conversations from store for ChatSidebar
  try {
    const chatsData = chatStore.conversations
    if (!chatsData) {
      console.log('DEBUG: chatsData is null/undefined')
      return []
    }

    const result = [...chatsData.values()].map((chat: any) => ({
      id: chat.id,
      project_id: chat.project_id || '', // From API if available
      user_id: chat.user_id || '', // From API if available
      title: chat.title || 'Untitled Chat',
      status: chat.status || 'completed', // Use API status or fallback
      created_at: chat.created_at || '', // From API if available
      updated_at: chat.updated_at || '', // From API if available
    }))

    console.log('DEBUG: Computed conversations result:', result)
    return result
  } catch (error) {
    console.error('Error in conversations computed:', error)
    return []
  }
})
const currentConversationId = computed(() => chatStore.currentConversationId || null)
const messages = computed(() => chatStore.messages || [])
const isLoading = computed(() => chatStore.isProcessing || false)
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
  console.log('DEBUG: sendMessage called, currentConversationId:', currentConversationId.value)

  if (currentConversationId.value) {
    await chatStore.sendMessage(content)
  } else {
    console.log('DEBUG: Creating new conversation with content:', content)
    // Create new conversation with initial message
    await chatStore.createConversation(undefined, content)
    console.log(
      'DEBUG: After createConversation, currentConversationId:',
      currentConversationId.value,
    )
  }
}

const createNewConversation = async () => {
  await chatStore.createConversation()
}

const selectConversation = async (conversationId: string) => {
  chatStore.selectConversation(conversationId)

  // 🚀 ENHANCED: Auto-scroll to bottom when selecting conversation
  await nextTick()
  if (chatMainRef.value?.messagesContainer) {
    setTimeout(() => {
      chatMainRef.value?.messagesContainer?.scrollTo({
        top: chatMainRef.value?.messagesContainer?.scrollHeight || 0,
        behavior: 'smooth',
      })
    }, 100)
  }
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
  if (
    isMobile.value &&
    sidebarOpen.value &&
    sidebarRef.value?.sidebar &&
    !sidebarRef.value.sidebar.contains(target) &&
    !target.closest('.mobile-menu-toggle') &&
    !target.closest('.settings-dropdown')
  ) {
    sidebarOpen.value = false
  }
}

// Watch for connection changes to load conversations
watch(isConnected, (connected) => {
  console.log('DEBUG: isConnected changed to:', connected)
  if (connected) {
    console.log('DEBUG: WebSocket connected, loading conversations...')
    chatStore.loadConversations().then(() => {
      // 🚀 ENHANCED: Auto-scroll after conversations load if we have a conversation selected
      if (currentConversationId.value) {
        setTimeout(() => {
          chatMainRef.value?.messagesContainer?.scrollTo({
            top: chatMainRef.value?.messagesContainer?.scrollHeight || 0,
            behavior: 'smooth',
          })
        }, 200)
      }
    })
  }
})

// Watch for route changes to handle conversation_id updates
watch(
  () => route.params.conversation_id as string,
  (newConversationId, oldConversationId) => {
    if (newConversationId && newConversationId !== oldConversationId) {
      console.log(
        'DEBUG: Route conversation_id changed from',
        oldConversationId,
        'to',
        newConversationId,
      )

      // Wait for connection and loading to complete, with longer timeout
      const selectWithRetry = () => {
        if (isConnected.value && !chatStore.isLoading) {
          console.log('DEBUG: Connection ready, loading conversation:', newConversationId)
          chatStore.loadConversations().then(() => {
            console.log('DEBUG: Conversations loaded, selecting:', newConversationId)
            chatStore.selectConversation(newConversationId)
          })
        } else {
          const timeout = chatStore.isLoading ? 200 : 100
          console.log(
            `DEBUG: Waiting for connection/load, retrying in ${timeout}ms... (connected=${isConnected.value}, loading=${chatStore.isLoading})`,
          )
          setTimeout(selectWithRetry, timeout)
        }
      }

      selectWithRetry()
    }
  },
  { immediate: true }, // Also run on mount
)

// Lifecycle
onMounted(async () => {
  console.log('🔥 ChatLayout mounted!')

  checkMobile()
  window.addEventListener('resize', checkMobile)
  document.addEventListener('click', handleClickOutside)

  // Initialize project and conversation from route params
  const projectId = route.params.id as string
  const conversationId = route.params.conversation_id as string
  console.log('Current project ID:', projectId)
  console.log('Current conversation ID from URL:', conversationId)

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

  // 🔄 NEW: Check for active streaming and mark as interrupted
  const currentConversation = chatStore.currentConversation
  if (currentConversation && currentConversation.status === 'processing') {
    console.log('🔌 Page unmounting with active streaming, marking as interrupted')

    // TODO: Send interruption signal when method is available
    // chatStore.sendInterruption(currentConversation.id)
  }

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
