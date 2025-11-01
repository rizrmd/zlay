<template>
  <button @click="$emit('click')"
    class="flex w-full items-center cursor-pointer px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 hover:text-gray-900">
    <slot />
  </button>
</template>

<script setup lang="ts">
defineEmits<{
  click: []
}>()
</script>