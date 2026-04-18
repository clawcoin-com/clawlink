<script setup lang="ts">
const props = withDefaults(defineProps<{
  size?: 'sm' | 'md' | 'lg' | 'xl'
  showWordmark?: boolean
  wordmarkClass?: string
}>(), {
  size: 'md',
  showWordmark: true,
  wordmarkClass: '',
})

const sizeClass = computed(() => {
  switch (props.size) {
    case 'sm': return 'h-5 w-auto'
    case 'lg': return 'h-10 w-auto'
    case 'xl': return 'h-14 w-auto'
    default: return 'h-7 w-auto'
  }
})

const wordmarkSizeClass = computed(() => {
  switch (props.size) {
    case 'sm': return 'text-sm'
    case 'lg': return 'text-2xl'
    case 'xl': return 'text-3xl'
    default: return 'text-lg'
  }
})
</script>

<template>
  <span class="inline-flex items-center gap-2.5 min-w-0">
    <img
      src="/icon-180x180.png"
      alt="ClawLink logo"
      class="block select-none"
      :class="sizeClass"
      draggable="false"
    >

    <span
      v-if="showWordmark"
      class="font-bold tracking-tight text-moltbook-teal truncate"
      :class="[wordmarkSizeClass, wordmarkClass]"
      style="font-family: 'IBM Plex Mono', monospace; letter-spacing: -0.02em;"
    >
      ClawLink
    </span>
  </span>
</template>
