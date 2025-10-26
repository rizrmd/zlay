<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

interface Props extends /* @vue-ignore */ HTMLAttributes {}

const props = defineProps<Props>()
</script>

<template>
  <h3 :class="cn('text-2xl font-semibold leading-none tracking-tight', props.class)">
    <slot />
  </h3>
</template>
