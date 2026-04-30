<script setup lang="ts">
// Renders a user avatar with the v0.4 fallback rule:
//   1. if `avatar` (URL) is set, render <img>
//   2. otherwise render a circular color block using `avatar_color`
//      with the first letter of display_name / username inside.
//
// We accept loose props so the component works for User, PublicUser,
// reply.author?, post.author? — anything with the same fields.

const props = defineProps<{
  url?: string | null
  color?: string | null
  name?: string | null
  size?: number // px, default 32
}>()

const px = computed(() => `${props.size ?? 32}px`)
const fontSize = computed(() => `${Math.round((props.size ?? 32) * 0.45)}px`)

const initial = computed(() => {
  const n = (props.name ?? '').trim()
  if (!n) return '?'
  // Use the first non-whitespace char (works for unicode/emoji/Chinese)
  const first = Array.from(n)[0]
  return first.toUpperCase()
})

const bg = computed(() => {
  const raw = (props.color ?? '').trim()
  if (raw && /^#[0-9a-fA-F]{6}$/.test(raw)) return raw
  return '#6b7280' // neutral grey fallback
})

const hasUrl = computed(() => {
  const u = (props.url ?? '').trim()
  return !!u
})
</script>

<template>
  <div
    class="rounded-full flex items-center justify-center overflow-hidden flex-shrink-0 select-none"
    :style="{ width: px, height: px, fontSize: fontSize, backgroundColor: bg, color: 'white' }"
  >
    <img
      v-if="hasUrl"
      :src="url!"
      :alt="name ?? 'avatar'"
      class="w-full h-full object-cover"
      referrerpolicy="no-referrer"
    />
    <span v-else class="font-bold leading-none">{{ initial }}</span>
  </div>
</template>

