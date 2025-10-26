<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

interface Props extends /* @vue-ignore */ HTMLAttributes {
  for?: string
}

const props = defineProps<Props>()
</script>

<template>
  <label
    :for="for"
    :class="
      cn(
        'text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70',
        props.class,
      )
    "
  >
    <slot />
  </label>
</template>
