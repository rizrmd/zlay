// Main layout component
export { default as ChatLayout } from './ChatLayout.vue'

// Individual components
export { default as ConnectionStatus } from './ConnectionStatus.vue'
export { default as MobileMenuToggle } from './MobileMenuToggle.vue'
export { default as ChatSidebarHeader } from './ChatSidebarHeader.vue'
export { default as ConversationList } from './ConversationList.vue'
export { default as ChatSidebar } from './ChatSidebar.vue'
export { default as WelcomeMessage } from './WelcomeMessage.vue'
export { default as MessageList } from './MessageList.vue'
export { default as LoadingIndicator } from './LoadingIndicator.vue'
export { default as MessageInput } from './MessageInput.vue'
export { default as ChatMain } from './ChatMain.vue'

// Composables
export { useMobileDetection } from './composables/useMobileDetection'
export { useScrollToBottom } from './composables/useScrollToBottom'
export { useChatActions } from './composables/useChatActions'

// Types
export type { Chat } from './types'
export { formatTime } from './types'