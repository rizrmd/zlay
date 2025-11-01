<template>
  <div class="flex h-screen bg-background">
    <!-- Connection Status -->
    <div v-if="!isConnected" class="fixed top-4 right-4 bg-yellow-100 text-yellow-800 px-4 py-2 rounded-lg z-50">
      Connecting to chat...
    </div>

    <!-- Mobile Menu Toggle -->
    <Button v-if="isMobile && !sidebarOpen" @click="toggleSidebar" variant="outline" size="icon"
      class="mobile-menu-toggle fixed top-4 left-4 z-50 md:hidden">
      <svg class="w-6 h-6" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
      </svg>
    </Button>

    <!-- Sidebar -->
    <div ref="sidebar" :class="[
      'bg-card border-r flex flex-col fixed md:relative h-full z-40 transition-transform duration-300',
      sidebarOpen ? 'translate-x-0' : '-translate-x-full',
      'w-80'
    ]">
      <!-- Top Navigation Bar -->
      <div class="p-2 pl-0 border-b">
        <div class="flex items-center justify-between">
          <Button variant="ghost" class="justify-start cursor-pointer" @click="navigateToDashboard">
            <svg class="w-4 h-4 " xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"
              stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
            </svg>
            Projects
          </Button>

          <div class="flex items-center space-x-2">
            <!-- New Chat Button -->
            <Button @click="createNewConversation" size="sm" :disabled="!isConnected">
              New chat
            </Button>
          </div>
        </div>
      </div>

      <!-- Conversations List -->
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

        <Card v-for="conversation in conversations" :key="conversation.id"
          class="cursor-pointer hover:bg-accent transition-colors"
          :class="{ 'ring-2 ring-primary': currentConversationId === conversation.id }"
          @click="selectConversation(conversation.id)">
          <CardContent class="flex items-center justify-between p-4">
            <div class="flex-1 min-w-0">
              <div class="font-medium text-sm truncate">{{ conversation.title }}</div>
              <div class="text-xs text-muted-foreground mt-1">
                {{ chatStore.formatMessageTime(conversation as any) }}
              </div>
            </div>
            <Button variant="ghost" size="icon" @click.stop="deleteConversation(conversation.id)">
              <svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"
                stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>

    <!-- Main Chat Area -->
    <div :class="[
      'flex flex-col flex-1 transition-all duration-300',
      sidebarOpen && isMobile ? 'ml-80' : 'ml-0'
    ]">
      <!-- Chat Messages -->
      <div ref="messagesContainer" class="flex-1 overflow-y-auto space-y-4">
        <!-- Welcome Message (shown when no messages) -->
        <div v-if="!hasMessages" class="text-center py-8">
          <div class="inline-flex items-center justify-center w-16 h-16 bg-primary/10 rounded-full mb-4">
            <svg class="w-8 h-8 text-primary" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"
              stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
            </svg>
          </div>
          <h2 class="text-2xl font-semibold text-foreground mb-2">How can I help you today?</h2>
          <p class="text-muted-foreground" v-if="isConnected">Start a new conversation or select an existing chat from
            the sidebar.</p>
          <p class="text-muted-foreground" v-else>Connecting to chat service...</p>
        </div>

        <!-- Chat Messages -->
        <div v-for="message in messages" :key="message.id" class="flex space-x-4"
          :class="message.role === 'user' ? 'justify-end' : 'justify-start'">
          <div v-if="message.role === 'assistant'" class="flex-shrink-0">
            <Avatar class="w-8 h-8 bg-primary">
              <AvatarFallback class="bg-primary text-primary-foreground">
                <svg class="w-5 h-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"
                  stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                </svg>
              </AvatarFallback>
            </Avatar>
          </div>

          <div class="max-w-2xl flex-1">
            <Card :class="message.role === 'user' ? 'bg-primary text-primary-foreground ml-auto' : 'bg-muted'">
              <CardContent class="p-4">
                <p class="whitespace-pre-wrap">{{ message.content }}</p>

                <!-- Tool calls indicator -->
                <div v-if="chatStore.isToolMessage(message)" class="mt-2 space-y-2">
                  <div v-for="(toolCall, index) in message.tool_calls" :key="index"
                    class="flex items-center space-x-2 text-sm">
                    <div class="w-2 h-2 rounded-full" :class="getToolStatusColor(chatStore.getToolStatus(toolCall))">
                    </div>
                    <span class="text-muted-foreground">
                      {{ toolCall.function.name }}
                      ({{ chatStore.getToolStatus(toolCall) }})
                    </span>
                  </div>
                </div>
              </CardContent>
            </Card>
            <div class="text-xs text-muted-foreground mt-1"
              :class="message.role === 'user' ? 'text-right' : 'text-left'">
              {{ chatStore.formatMessageTime(message) }}
            </div>
          </div>

          <div v-if="message.role === 'user'" class="flex-shrink-0">
            <Avatar class="w-8 h-8 bg-secondary">
              <AvatarFallback class="bg-secondary text-secondary-foreground">
                <svg class="w-5 h-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"
                  stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                </svg>
              </AvatarFallback>
            </Avatar>
          </div>
        </div>

        <!-- Loading indicator -->
        <div v-if="isLoading" class="flex space-x-4 justify-start">
          <div class="flex-shrink-0">
            <Avatar class="w-8 h-8 bg-primary">
              <AvatarFallback class="bg-primary text-primary-foreground">
                <svg class="w-5 h-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"
                  stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                </svg>
              </AvatarFallback>
            </Avatar>
          </div>
          <Card class="bg-muted">
            <CardContent class="p-4">
              <div class="flex space-x-1">
                <div class="w-2 h-2 bg-muted-foreground rounded-full animate-bounce"></div>
                <div class="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style="animation-delay: 0.1s">
                </div>
                <div class="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style="animation-delay: 0.2s">
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>

      <!-- Chat Input -->
      <div class="border-t p-4">
        <div class="max-w-4xl mx-auto">
          <div class="flex items-end space-x-4">
            <div class="flex-1">
              <Textarea v-model="currentMessage" @keydown="handleKeyDown" @input="autoResize" ref="messageInput"
                placeholder="Type your message..." :rows="1" :disabled="isLoading || !isConnected"
                class="resize-none" />
            </div>
            <Button @click="sendMessage" :disabled="!currentMessage.trim() || isLoading || !isConnected">
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
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Textarea } from '@/components/ui/textarea'
import { useChatStore } from '@/stores/chat'
import { useAuthStore } from '@/stores/auth'

// Get stores
const chatStore = useChatStore()
const authStore = useAuthStore()

// Connection and conversation state
const isConnected = ref(false)
const conversations = ref<any[]>([])
const currentConversationId = ref<string | null>(null)

interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  timestamp: Date
}

interface Chat {
  id: string
  title: string
  lastMessage: string
  timestamp: Date
}

const route = useRoute()
const router = useRouter()

const openSection = ref('chats')
const sidebarOpen = ref(true)
const isMobile = ref(false)
const isLoading = ref(false)
const currentMessage = ref('')
const messages = ref<Message[]>([])
const chats = ref<Chat[]>([
  {
    id: '1',
    title: 'Chat about project requirements',
    lastMessage: 'Let me help you understand the requirements...',
    timestamp: new Date(Date.now() - 2 * 60 * 60 * 1000) // 2 hours ago
  },
  {
    id: '2',
    title: 'Database schema discussion',
    lastMessage: 'The schema looks good for now...',
    timestamp: new Date(Date.now() - 24 * 60 * 60 * 1000) // Yesterday
  },
  {
    id: '3',
    title: 'API design review',
    lastMessage: 'Consider adding pagination to the endpoints...',
    timestamp: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000) // 2 days ago
  }
])

const sidebar = ref<HTMLElement | null>(null)
const messagesContainer = ref<HTMLElement | null>(null)
const messageInput = ref<HTMLTextAreaElement | null>(null)
const showSettingsMenu = ref(false)

// Computed properties
const hasMessages = computed(() => messages.value.length > 0)

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

const toggleSection = (section: string) => {
  // Only change if clicking a different section
  if (openSection.value !== section) {
    openSection.value = section
  }
  // If clicking the same active section, do nothing (keep it open)
}

const toggleSettingsMenu = () => {
  showSettingsMenu.value = !showSettingsMenu.value
}

const refreshDatasources = () => {
  console.log('Refreshing datasources...')
  // TODO: Implement actual datasource refresh
}

const formatTime = (date: Date) => {
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const hours = Math.floor(diff / (1000 * 60 * 60))
  const days = Math.floor(hours / 24)

  if (hours < 1) return 'Just now'
  if (hours < 24) return `${hours}h ago`
  if (days < 7) return `${days}d ago`
  return date.toLocaleDateString()
}

const sendMessage = async () => {
  if (!currentMessage.value.trim() || isLoading.value) return

  const userMessage: Message = {
    id: Date.now().toString(),
    role: 'user',
    content: currentMessage.value.trim(),
    timestamp: new Date()
  }

  messages.value.push(userMessage)
  const messageContent = currentMessage.value.trim()
  currentMessage.value = ''
  isLoading.value = true

  // Scroll to bottom
  await nextTick()
  scrollToBottom()

  // Simulate API call
  setTimeout(() => {
    const assistantMessage: Message = {
      id: (Date.now() + 1).toString(),
      role: 'assistant',
      content: `I understand you said: "${messageContent}". This is a simulated response since we haven't connected to the backend yet. The actual chat functionality will be implemented once we add the API endpoints.`,
      timestamp: new Date()
    }

    messages.value.push(assistantMessage)
    isLoading.value = false

    // Scroll to bottom again
    nextTick(() => scrollToBottom())
  }, 1500)
}

const scrollToBottom = () => {
  if (messagesContainer.value) {
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  }
}

const autoResize = () => {
  if (messageInput.value) {
    messageInput.value.style.height = 'auto'
    messageInput.value.style.height = Math.min(messageInput.value.scrollHeight, 120) + 'px'
  }
}

const handleKeyDown = (event: KeyboardEvent) => {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    sendMessage()
  }
}

const selectChat = (chatId: string) => {
  // TODO: Load chat messages
  console.log('Selected chat:', chatId)
}

const createNewConversation = () => {
  // TODO: Create new conversation
  console.log('Creating new conversation...')
  currentConversationId.value = null
  messages.value = []
}

const selectConversation = (conversationId: string) => {
  currentConversationId.value = conversationId
  // TODO: Load conversation messages
  console.log('Selected conversation:', conversationId)
}

const deleteConversation = async (conversationId: string) => {
  if (confirm('Are you sure you want to delete this conversation?')) {
    // TODO: Implement delete conversation
    console.log('Deleting conversation:', conversationId)
  }
}

const getToolStatusColor = (status: string) => {
  switch (status) {
    case 'completed':
      return 'bg-green-500'
    case 'running':
      return 'bg-yellow-500'
    case 'failed':
      return 'bg-red-500'
    default:
      return 'bg-gray-500'
  }
}

const navigateToDashboard = () => {
  router.push('/dashboard')
}

const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (isMobile.value &&
    sidebarOpen.value &&
    sidebar.value &&
    !sidebar.value.contains(target) &&
    !target.closest('.mobile-menu-toggle') &&
    !target.closest('.settings-dropdown')) {
    sidebarOpen.value = false
  }

  if (showSettingsMenu.value && !target.closest('.settings-dropdown')) {
    showSettingsMenu.value = false
  }
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  document.addEventListener('click', handleClickOutside)

  // Initialize project from route params
  const projectId = route.params.id as string
  console.log('Current project ID:', projectId)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
  document.removeEventListener('click', handleClickOutside)
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

/* Accordion animations */
.accordion-enter-active,
.accordion-leave-active {
  transition: all 0.3s ease-out;
  overflow: hidden;
}

.accordion-enter-from {
  max-height: 0;
  opacity: 0;
  transform: translateY(-10px);
}

.accordion-leave-to {
  max-height: 0;
  opacity: 0;
  transform: translateY(-10px);
}

.accordion-enter-active .accordion-enter-from {
  max-height: 500px;
  opacity: 1;
  transform: translateY(0);
}

.accordion-leave-active .accordion-leave-to {
  max-height: 500px;
  opacity: 1;
  transform: translateY(0);
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