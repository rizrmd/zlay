<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

interface Props extends /* @vue-ignore */ HTMLAttributes {}

const props = defineProps<Props>()
</script>

<template>
  <div :class="cn('rounded-lg border bg-card text-card-foreground shadow-sm', props.class)">
    <slot />
  </div>
</template>
