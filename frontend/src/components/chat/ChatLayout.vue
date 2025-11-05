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
import { useProjectStore } from '@/stores/project'
import webSocketService from '@/services/websocket'
import ConnectionStatus from '@/components/chat/ConnectionStatus.vue'
import MobileMenuToggle from '@/components/chat/MobileMenuToggle.vue'
import ChatSidebar from '@/components/chat/ChatSidebar.vue'
import ChatMain from '@/components/chat/ChatMain.vue'
import type { Conversation } from '@/services/websocket'

// Stores
const chatStore = useChatStore()
const projectStore = useProjectStore()

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

    return result
  } catch (error) {
    console.error('Error in conversations computed:', error)
    return []
  }
})
const currentConversationId = computed(() => chatStore.currentConversationId || null)
const messages = computed(() => {
  console.log('📊 CHAT LAYOUT: Messages computed, count:', chatStore.messages.length)
  console.log('📊 CHAT LAYOUT: Sample message:', chatStore.messages[0])
  return chatStore.messages
})
const isLoading = computed(() => {
  console.log('📊 CHAT LAYOUT: isLoading computed:', chatStore.isLoading)
  return chatStore.isLoading || false
})
const hasMessages = computed(() => {
  console.log('📊 CHAT LAYOUT: hasMessages computed:', chatStore.messages.length > 0)
  return chatStore.messages.length > 0
})

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
    // 🚀 NEW: Ensure we're on the correct conversation URL
    const projectId = route.params.id as string
    const currentConversationIdFromUrl = route.params.conversation_id as string

    if (currentConversationIdFromUrl !== currentConversationId.value) {
      console.log('🚀 Updating URL to match current conversation:', currentConversationId.value)
      await router.replace(`/p/${projectId}/chat/${currentConversationId.value}`)
    }

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
  console.log('🔄 CHAT SWITCH: Selecting conversation', conversationId)
  chatStore.selectConversation(conversationId)

  // 🚀 ENHANCED: Auto-scroll to bottom when selecting conversation
  await nextTick()
  if (chatMainRef.value?.messagesContainer) {
    setTimeout(() => {
      chatMainRef.value?.messagesContainer?.scrollTo({
        top: chatMainRef.value?.messagesContainer?.scrollHeight || 0,
        behavior: 'smooth',
      })
    }, 500)
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
// watch(isConnected, (connected) => {
//   console.log('🏗️ PROJECT STORE: isConnected changed to:', connected)
//   console.log('🏗️ PROJECT STORE: Current state:', {
//     conversationsSize: chatStore.conversations.size,
//     isLoadingConversations: chatStore.isLoadingConversations,
//   })

//   if (connected) {
//     console.log('🏗️ PROJECT STORE: WebSocket connected, evaluating conversation loading...')

//     // 🔄 CRITICAL FIX: Get conversation ID from route, not currentConversationId.value
//     const routeConversationId = route.params.conversation_id as string
//     console.log('🏗️ PROJECT STORE: Route conversation ID:', routeConversationId)
//     webSocketService.requestStreamingConversation(routeConversationId)
// console.log('🏗️ PROJECT STORE: currentConversationId.value:', currentConversationId.value)

// // Only load if conversations aren't already loading AND not already loaded
// if (!chatStore.isLoadingConversations && chatStore.conversations.size === 0) {
//   console.log('🏗️ PROJECT STORE: Loading conversations (conditions met)')
//   chatStore.loadConversations().then(() => {
//     // After loading conversations, check if current conversation is processing
//     console.log('🏗️ PROJECT STORE: Load Conv for id: ', routeConversationId)
//     if (routeConversationId) {
//       const conversation = chatStore.conversations.get(routeConversationId)
//       const isProcessing = conversation?.status === 'processing'

//       console.log(
//         '🏗️ PROJECT STORE: After loading conversations, checking current conversation:',
//         {
//           conversationId: routeConversationId,
//           status: conversation?.status,
//           isProcessing,
//         },
//       )

//       if (isProcessing) {
//         // Request streaming state for processing conversation (page refresh case)
//         console.log('🔄 Requesting streaming state for current processing conversation:', routeConversationId)
//         webSocketService.requestStreamingConversation(routeConversationId)
//       }

//       setTimeout(() => {
//         chatMainRef.value?.messagesContainer?.scrollTo({
//           top: chatMainRef.value?.messagesContainer?.scrollHeight || 0,
//           behavior: 'smooth',
//         })
//       }, 200)
//     }
//   })
// }
// else {
//   console.log('🏗️ PROJECT STORE: Skipping conversation load:', {
//     isLoadingConversations: chatStore.isLoadingConversations,
//     conversationsSize: chatStore.conversations.size,
//   })
// }
//   }
// })

// Watch for route changes to handle conversation_id updates
watch(
  () => route.params.conversation_id as string,
  (newConversationId, oldConversationId) => {
    if (newConversationId && newConversationId !== oldConversationId) {
      console.log(
        '💬 CHAT STORE: Route conversation_id changed from',
        oldConversationId,
        'to',
        newConversationId,
      )

      const selectWithRetry = () => {
        if (isConnected.value) {
          console.log('💬 CHAT STORE: Connection ready, loading conversation:', newConversationId)

          // Only load conversations if they're not already loading or loaded
          if (!chatStore.isLoadingConversations && chatStore.conversations.size === 0) {
            console.log('💬 CHAT STORE: Loading conversations for route change')
            chatStore.loadConversations().then(() => {
              // After loading conversations, check if current one is processing
              const conversation = chatStore.conversations.get(newConversationId)
              const isProcessing = conversation?.status === 'processing'

              console.log(
                '💬 CHAT STORE: After loading conversations, checking processing state22:',
                {
                  conversationId: newConversationId,
                  status: conversation?.status,
                  isProcessing,
                },
              )

              if (isProcessing) {
                // For processing conversations, request streaming state instead of database state
                console.log(
                  '🔄 Requesting streaming state for processing conversation:',
                  newConversationId,
                )
                // webSocketService.requestStreamingConversation(newConversationId)
              } else {
                // Load messages for completed conversations
                console.log('💬 CHAT STORE: Loading conversation messages:', newConversationId)
                chatStore.loadConversation(newConversationId).then(() => {
                  chatStore.selectConversation(newConversationId)
                })
              }
            })
          } else {
            // Conversations already loaded, just proceed with selection
            console.log(
              '💬 CHAT STORE: Conversations already loaded, proceeding with selection222',
              {
                conversationsSize: chatStore.conversations.size,
                isLoadingConversations: chatStore.isLoadingConversations,
              },
            )
            // const conversation = chatStore.conversations.get(newConversationId)
            // const isProcessing = conversation?.status === 'processing'
            // console.log('💬 CHAT STORE: processing state: ', conversation)

            // if (isProcessing) {
            //   chatStore.selectConversation(newConversationId)
            // } else {
            webSocketService.requestStreamingConversation(newConversationId) // Check streaming connection
            // Load conversation
            chatStore.loadConversation(newConversationId).then(() => {
              chatStore.selectConversation(newConversationId)
            })
            // }
          }
        } else {
          console.log('💬 CHAT STORE: Waiting for connection, retrying in 100ms...')
          setTimeout(selectWithRetry, 100)
        }
      }

      selectWithRetry()
    }
  },
  { immediate: true },
)

// 🚀 NEW: Handle base chat path redirection (when no conversation_id provided)
watch(
  () => route.params.conversation_id,
  (conversationId) => {
    const projectId = route.params.id as string

    // If we're on base chat path (/p/:id/chat) and no conversation_id
    if (projectId && !conversationId && isConnected.value) {
      console.log('🚀 Base chat path detected, handling redirection...')
      chatStore.handleBaseChatPath(projectId)
    }
  },
  { immediate: true },
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
  console.log('🏗️ PROJECT STORE: Current project ID:', projectId)
  console.log('💬 CHAT STORE: Current conversation ID from URL:', conversationId)

  // Initialize chat system
  if (projectId) {
    try {
      await chatStore.initChat(projectId)
    } catch (error) {
      console.error('Failed to initialize chat system:', error)
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
