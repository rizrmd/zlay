<script setup lang="ts">
import { AlertDialogRoot } from 'reka-ui'

const props = defineProps()
</script>

<template>
  <AlertDialogRoot v-bind="props">
    <slot />
  </AlertDialogRoot>
</template>