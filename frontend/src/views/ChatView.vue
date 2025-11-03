<template>
  <ChatLayout />
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import ChatLayout from '@/components/chat/ChatLayout.vue'

onMounted(() => {
  console.log('🚀 ChatView mounted!')
})
</script>