<template>
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
</template>

<script setup lang="ts">
interface Props {
  hasMessages: boolean
  isConnected: boolean
}

defineProps<Props>()
</script>