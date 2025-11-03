<template>
  <div
    v-for="message in messages"
    :key="message.id"
    class="flex space-x-4"
    :class="message.role === 'user' ? 'justify-end' : 'justify-start'"
  >
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
          <div v-if="isToolMessage(message)" class="mt-2 space-y-2">
            <div
              v-for="(toolCall, index) in message.tool_calls"
              :key="index"
              class="border rounded-md p-3 bg-muted/50"
            >
              <div class="flex items-center space-x-2 text-sm mb-2">
                <div class="w-2 h-2 rounded-full" :class="getToolStatusColor(chatStore.getToolStatus(toolCall))">
                </div>
                <span class="font-medium">
                  {{ toolCall.function.name }}
                </span>
                <span class="text-muted-foreground text-xs">
                  ({{ chatStore.getToolStatus(toolCall) }})
                </span>
              </div>
              
              <!-- Tool arguments -->
              <div v-if="toolCall.function.arguments" class="text-xs text-muted-foreground mb-2">
                <span class="font-semibold">Arguments:</span>
                <pre class="mt-1 p-2 bg-background rounded border text-xs overflow-auto">{{ JSON.stringify(toolCall.function.arguments, null, 2) }}</pre>
              </div>
              
              <!-- Tool result (when completed) -->
              <div v-if="toolCall.status === 'completed' && toolCall.result" class="text-xs">
                <span class="font-semibold text-green-600">Result:</span>
                <pre class="mt-1 p-2 bg-green-50 dark:bg-green-950/20 rounded border text-xs overflow-auto max-h-48">{{ 
                  typeof toolCall.result === 'string' ? toolCall.result : JSON.stringify(toolCall.result, null, 2) 
                }}</pre>
              </div>
              
              <!-- Tool error (when failed) -->
              <div v-if="toolCall.status === 'failed' && toolCall.error" class="text-xs">
                <span class="font-semibold text-red-600">Error:</span>
                <pre class="mt-1 p-2 bg-red-50 dark:bg-red-950/20 rounded border text-xs overflow-auto">{{ toolCall.error }}</pre>
              </div>
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
</template>

<script setup lang="ts">
import { Card, CardContent } from '@/components/ui/card'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import type { ChatMessage, ToolCall } from '@/services/websocket'
import { useChatStore } from '@/stores/chat'

interface Props {
  messages: ChatMessage[]
}

defineProps<Props>()

const chatStore = useChatStore()

const isToolMessage = (message: ChatMessage) => {
  return message.tool_calls && message.tool_calls.length > 0
}

const getToolStatusColor = (status: string) => {
  switch (status) {
    case 'completed':
      return 'bg-green-500'
    case 'executing':
    case 'running':
      return 'bg-yellow-500 animate-pulse'
    case 'failed':
      return 'bg-red-500'
    case 'pending':
      return 'bg-gray-500'
    default:
      return 'bg-gray-500'
  }
}
</script>