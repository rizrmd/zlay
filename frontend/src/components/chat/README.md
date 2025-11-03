# Chat Components

This directory contains the modularized chat components for the Zlay platform.

## Structure

### Main Layout
- **`ChatLayout.vue`** - Main container that orchestrates all chat components

### UI Components
- **`ConnectionStatus.vue`** - Shows WebSocket connection status
- **`MobileMenuToggle.vue`** - Mobile hamburger menu button
- **`ChatSidebarHeader.vue`** - Header with navigation and new chat button
- **`ConversationList.vue`** - List of chat conversations
- **`ChatSidebar.vue`** - Complete sidebar component
- **`WelcomeMessage.vue`** - Welcome message when no messages exist
- **`MessageList.vue`** - List of chat messages with tool call indicators
- **`LoadingIndicator.vue`** - Animated loading indicator
- **`MessageInput.vue`** - Chat input with textarea and send button
- **`ChatMain.vue`** - Main chat area with messages and input

### Composables
- **`useMobileDetection.ts`** - Mobile viewport detection and sidebar management
- **`useScrollToBottom.ts`** - Auto-scroll to bottom functionality
- **`useChatActions.ts`** - Chat action methods (send, create, delete conversations)

### Types & Utilities
- **`types.ts`** - TypeScript interfaces and utility functions
- **`index.ts`** - Barrel export for easy imports

## Usage

```vue
<template>
  <ChatLayout />
</template>

<script setup lang="ts">
import { ChatLayout } from '@/components/chat'
</script>
```

Or import individual components:

```vue
<script setup lang="ts">
import {
  MessageList,
  MessageInput,
  ConnectionStatus
} from '@/components/chat'
</script>
```

## Features

### Responsive Design
- Mobile-first approach with collapsible sidebar
- Automatic mobile detection
- Touch-friendly interface

### Real-time Communication
- WebSocket integration
- Connection status indicators
- Auto-reconnection handling

### Chat Functionality
- Message sending and receiving
- Tool call indicators
- Conversation management
- Typing indicators

### Accessibility
- Proper ARIA labels
- Keyboard navigation
- Screen reader support

## Styling

All components use Tailwind CSS with shadcn-vue design system. Custom animations are defined in component `<style>` blocks with scoped styles.

## State Management

Components integrate with Pinia stores:
- `useChatStore` - Chat state and WebSocket management
- `useAuthStore` - Authentication state