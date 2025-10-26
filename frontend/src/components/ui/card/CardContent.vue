<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

interface Props extends /* @vue-ignore */ HTMLAttributes {}

const props = defineProps<Props>()
</script>

<template>
  <div :class="cn('p-6 pt-0', props.class)">
    <slot />
  </div>
</template>
