<template>
  <div class="flex h-screen bg-background">
    <!-- Mobile Menu Toggle -->
    <Button 
      v-if="isMobile && !sidebarOpen"
      @click="toggleSidebar"
      variant="outline"
      size="icon"
      class="mobile-menu-toggle fixed top-4 left-4 z-50 md:hidden"
    >
      <svg class="w-6 h-6" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
      </svg>
    </Button>

    <!-- Sidebar -->
    <div 
      ref="sidebar"
      :class="[
        'bg-card border-r flex flex-col fixed md:relative h-full z-40 transition-transform duration-300',
        sidebarOpen ? 'translate-x-0' : '-translate-x-full',
        'w-80'
      ]"
    >
      <!-- Top Navigation Bar -->
      <CardHeader class="border-b">
        <div class="flex items-center justify-between">
          <Button variant="ghost" class="justify-start" @click="navigateToDashboard">
            <svg class="w-4 h-4 mr-2" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
            </svg>
            Projects
          </Button>
          
          <div class="flex items-center space-x-2">
            <!-- Project Settings Dropdown -->
            <div class="relative settings-dropdown">
              <Button variant="ghost" size="icon" @click="toggleSettingsMenu">
                <svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
                </svg>
              </Button>
              <!-- Settings Dropdown Menu -->
              <div 
                v-show="showSettingsMenu"
                class="absolute right-0 mt-1 w-48 bg-popover rounded-lg shadow-lg border border-border py-1 z-50"
                @click.stop
              >
                <button 
                  @click="() => { showSettingsMenu = false; console.log('Add member clicked') }"
                  class="w-full text-left px-4 py-2 text-sm hover:bg-accent transition-colors"
                >
                  Add Member
                </button>
                <button 
                  @click="() => { showSettingsMenu = false; console.log('Context clicked') }"
                  class="w-full text-left px-4 py-2 text-sm hover:bg-accent transition-colors"
                >
                  Context
                </button>
              </div>
            </div>
            
            <!-- New Chat Button -->
            <Button @click="createNewChat" size="sm">
              New chat
            </Button>
          </div>
        </div>
      </CardHeader>

      <!-- Accordion Sidebar -->
      <div class="flex-1 overflow-y-auto">
        <!-- Chats Section -->
        <div class="border-b">
          <Button 
            variant="ghost"
            class="w-full justify-between p-4 h-auto"
            @click="toggleSection('chats')"
          >
            <div class="flex items-center space-x-2">
              <svg class="w-4 h-4 transition-transform" :class="openSection === 'chats' ? 'rotate-90' : ''" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
              </svg>
              <span class="font-medium">Chats</span>
            </div>
          </Button>
          <transition
            name="accordion"
            enter-active-class="accordion-enter-active"
            leave-active-class="accordion-leave-active"
            enter-from-class="accordion-enter-from"
            leave-to-class="accordion-leave-to"
          >
            <div v-show="openSection === 'chats'" class="p-2 space-y-1">
              <Card 
                v-for="chat in chats" 
                :key="chat.id"
                class="cursor-pointer hover:bg-accent transition-colors"
                @click="selectChat(chat.id)"
              >
                <CardContent class="flex items-center justify-center min-h-[80px]">
                  <div class="w-full text-left space-y-1">
                    <div class="font-medium text-sm leading-tight">{{ chat.title }}</div>
                    <div class="text-xs text-muted-foreground">{{ formatTime(chat.timestamp) }}</div>
                  </div>
                </CardContent>
              </Card>
            </div>
          </transition>
        </div>

        <!-- Datasources Section -->
        <div class="border-b">
          <Button 
            variant="ghost"
            class="w-full justify-between p-4 h-auto"
            @click="toggleSection('datasources')"
          >
            <div class="flex items-center space-x-2">
              <svg class="w-4 h-4 transition-transform" :class="openSection === 'datasources' ? 'rotate-90' : ''" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
              </svg>
              <span class="font-medium">Datasources</span>
            </div>
            <Button 
              variant="ghost"
              size="icon"
              class="h-6 w-6"
              @click.stop="refreshDatasources"
            >
              <svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </Button>
          </Button>
          <transition
            name="accordion"
            enter-active-class="accordion-enter-active"
            leave-active-class="accordion-leave-active"
            enter-from-class="accordion-enter-from"
            leave-to-class="accordion-leave-to"
          >
            <div v-show="openSection === 'datasources'" class="p-2 space-y-1">
              <Card class="bg-muted/50">
                <CardContent class="flex items-center justify-between min-h-[80px]">
                  <div class="flex-1 space-y-1">
                    <div class="font-medium text-sm leading-tight">Production Database</div>
                    <div class="text-xs text-muted-foreground">PostgreSQL</div>
                  </div>
                  <div class="w-2 h-2 bg-green-500 rounded-full flex-shrink-0 ml-3"></div>
                </CardContent>
              </Card>
              <Card class="bg-muted/50">
                <CardContent class="flex items-center justify-between min-h-[80px]">
                  <div class="flex-1 space-y-1">
                    <div class="font-medium text-sm leading-tight">Analytics API</div>
                    <div class="text-xs text-muted-foreground">REST API</div>
                  </div>
                  <div class="w-2 h-2 bg-green-500 rounded-full flex-shrink-0 ml-3"></div>
                </CardContent>
              </Card>
              <Card class="bg-muted/50">
                <CardContent class="flex items-center justify-between min-h-[80px]">
                  <div class="flex-1 space-y-1">
                    <div class="font-medium text-sm leading-tight">File Storage</div>
                    <div class="text-xs text-muted-foreground">S3 Bucket</div>
                  </div>
                  <div class="w-2 h-2 bg-yellow-500 rounded-full flex-shrink-0 ml-3"></div>
                </CardContent>
              </Card>
            </div>
          </transition>
        </div>

        <!-- Analysis Section -->
        <div>
          <Button 
            variant="ghost"
            class="w-full justify-between p-4 h-auto"
            @click="toggleSection('analysis')"
          >
            <div class="flex items-center space-x-2">
              <svg class="w-4 h-4 transition-transform" :class="openSection === 'analysis' ? 'rotate-90' : ''" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
              </svg>
              <span class="font-medium">Analysis</span>
            </div>
          </Button>
          <transition
            name="accordion"
            enter-active-class="accordion-enter-active"
            leave-active-class="accordion-leave-active"
            enter-from-class="accordion-enter-from"
            leave-to-class="accordion-leave-to"
          >
            <div v-show="openSection === 'analysis'" class="p-2 space-y-1">
              <Card class="cursor-pointer hover:bg-accent transition-colors">
                <CardContent class="flex items-center justify-center min-h-[80px]">
                  <div class="w-full text-left space-y-1">
                    <div class="font-medium text-sm leading-tight">Q3 Performance Report</div>
                    <div class="text-xs text-muted-foreground">Generated 1 hour ago</div>
                  </div>
                </CardContent>
              </Card>
              <Card class="cursor-pointer hover:bg-accent transition-colors">
                <CardContent class="flex items-center justify-center min-h-[80px]">
                  <div class="w-full text-left space-y-1">
                    <div class="font-medium text-sm leading-tight">User Behavior Analysis</div>
                    <div class="text-xs text-muted-foreground">Generated yesterday</div>
                  </div>
                </CardContent>
              </Card>
            </div>
          </transition>
        </div>
      </div>
    </div>

    <!-- Main Chat Area -->
    <div 
      :class="[
        'flex flex-col transition-all duration-300',
        sidebarOpen && isMobile ? 'ml-80' : 'ml-0',
        !isMobile ? 'ml-80' : 'ml-0'
      ]"
    >
      <!-- Chat Messages -->
      <div ref="messagesContainer" class="flex-1 overflow-y-auto p-6 space-y-4">
        <!-- Welcome Message (shown when no messages) -->
        <div v-if="messages.length === 0" class="text-center py-8">
          <div class="inline-flex items-center justify-center w-16 h-16 bg-primary/10 rounded-full mb-4">
            <svg class="w-8 h-8 text-primary" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
            </svg>
          </div>
          <h2 class="text-2xl font-semibold text-foreground mb-2">How can I help you today?</h2>
          <p class="text-muted-foreground">Start a new conversation or select an existing chat from the sidebar.</p>
        </div>

        <!-- Chat Messages -->
        <div v-for="message in messages" :key="message.id" class="flex space-x-4" :class="message.role === 'user' ? 'justify-end' : 'justify-start'">
          <div v-if="message.role === 'assistant'" class="flex-shrink-0">
            <Avatar class="w-8 h-8 bg-primary">
              <AvatarFallback class="bg-primary text-primary-foreground">
                <svg class="w-5 h-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                </svg>
              </AvatarFallback>
            </Avatar>
          </div>
          
          <div class="max-w-2xl">
            <Card :class="message.role === 'user' ? 'bg-primary text-primary-foreground' : 'bg-muted'">
              <CardContent class="p-4">
                <p class="whitespace-pre-wrap">{{ message.content }}</p>
              </CardContent>
            </Card>
            <div class="text-xs text-muted-foreground mt-1" :class="message.role === 'user' ? 'text-right' : 'text-left'">
              {{ formatTime(message.timestamp) }}
            </div>
          </div>

          <div v-if="message.role === 'user'" class="flex-shrink-0">
            <Avatar class="w-8 h-8 bg-secondary">
              <AvatarFallback class="bg-secondary text-secondary-foreground">
                <svg class="w-5 h-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
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
                <svg class="w-5 h-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                </svg>
              </AvatarFallback>
            </Avatar>
          </div>
          <Card class="bg-muted">
            <CardContent class="p-4">
              <div class="flex space-x-1">
                <div class="w-2 h-2 bg-muted-foreground rounded-full animate-bounce"></div>
                <div class="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style="animation-delay: 0.1s"></div>
                <div class="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style="animation-delay: 0.2s"></div>
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
              <Textarea
                v-model="currentMessage"
                @keydown="handleKeyDown"
                @input="autoResize"
                ref="messageInput"
                placeholder="Type your message..."
                :rows="1"
                :disabled="isLoading"
                class="resize-none"
              />
            </div>
            <Button 
              @click="sendMessage"
              :disabled="!currentMessage.trim() || isLoading"
            >
              <svg v-if="!isLoading" class="w-4 h-4 mr-2" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
              </svg>
              <svg v-else class="w-4 h-4 mr-2 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
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
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Textarea } from '@/components/ui/textarea'


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

const createNewChat = () => {
  // TODO: Create new chat
  console.log('Creating new chat...')
  messages.value = []
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
  0%, 80%, 100% {
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