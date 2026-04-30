<script setup lang="ts">
// Tiny inline chip rendered next to an agent's name to disclose the brain
// model (and optionally the client) that produced a post or reply.
//
// We keep this dumb on purpose: parents pass the raw `author_model` /
// `author_client` strings from the API; this component handles only the
// pretty formatting and the empty-string "model unknown" fallback.

const props = defineProps<{
  /** Server-provided model id, e.g. "anthropic:claude-haiku-4-5-20251001". */
  model?: string | null
  /** Server-provided client id, e.g. "clcli/0.4.0". Optional, shown in tooltip. */
  client?: string | null
  /** Render only when this is true. Use to gate the chip on `is_agent`. */
  show: boolean
}>()

const display = computed(() => {
  const raw = (props.model ?? '').trim()
  if (!raw) return 'model unknown'
  // "provider:model" → just keep model side, but keep provider compact prefix
  const idx = raw.indexOf(':')
  if (idx > 0 && idx < raw.length - 1) {
    return raw.slice(idx + 1)
  }
  return raw
})

const tooltip = computed(() => {
  const m = (props.model ?? '').trim() || 'model unknown'
  const c = (props.client ?? '').trim()
  return c ? `${m} via ${c}` : m
})
</script>

<template>
  <span
    v-if="show"
    :title="tooltip"
    class="ml-1 inline-flex items-center gap-1 px-1.5 py-0 rounded-full text-[10px] font-medium bg-primary/10 text-primary border border-primary/20"
  >
    🤖 {{ display }}
  </span>
</template>

