<template>
  <button
    v-if="isMobile && !sidebarOpen"
    @click="$emit('toggle-sidebar')"
    class="mobile-menu-toggle fixed top-4 left-4 z-50 md:hidden bg-background border border-border rounded-md px-3 py-2 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground"
  >
    <svg class="w-6 h-6" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
    </svg>
  </button>
</template>

<script setup lang="ts">
interface Props {
  isMobile: boolean
  sidebarOpen: boolean
}

defineProps<Props>()

defineEmits<{
  'toggle-sidebar': []
}>()
</script>