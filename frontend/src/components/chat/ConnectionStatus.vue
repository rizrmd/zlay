<template>
  <div v-if="!isConnected" class="fixed top-4 right-4 bg-yellow-100 text-yellow-800 px-4 py-2 rounded-lg z-50">
    Connecting to chat...
  </div>
</template>

<script setup lang="ts">
interface Props {
  isConnected: boolean
}

defineProps<Props>()
</script>