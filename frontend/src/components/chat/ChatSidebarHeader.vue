<template>
  <div class="p-2 pl-0 border-b">
    <div class="flex items-center justify-between">
      <Button variant="ghost" class="justify-start cursor-pointer" @click="$emit('navigate-dashboard')">
        <svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"
          stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
        </svg>
        Projects
      </Button>

      <div class="flex items-center space-x-2">
        <Button @click="$emit('create-conversation')" size="sm" :disabled="!isConnected">
          New chat
        </Button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Button } from '@/components/ui/button'

interface Props {
  isConnected: boolean
}

defineProps<Props>()

defineEmits<{
  'navigate-dashboard': []
  'create-conversation': []
}>()
</script>