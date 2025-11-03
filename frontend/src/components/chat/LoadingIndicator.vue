<template>
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
</template>

<script setup lang="ts">
import { Card, CardContent } from '@/components/ui/card'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'

interface Props {
  isLoading: boolean
}

defineProps<Props>()
</script>