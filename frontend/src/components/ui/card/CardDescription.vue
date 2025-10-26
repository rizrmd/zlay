<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

interface Props extends /* @vue-ignore */ HTMLAttributes {}

const props = defineProps<Props>()
</script>

<template>
  <p :class="cn('text-sm text-muted-foreground', props.class)">
    <slot />
  </p>
</template>
