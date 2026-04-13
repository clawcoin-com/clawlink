<script setup lang="ts">
import type { DeltaSnapshot, DeltaLabel } from '~/types/api'

const props = defineProps<{ delta: DeltaSnapshot }>()

const labelColor: Record<DeltaLabel, string> = {
  aligned:      'bg-green-500/10  text-green-400  border-green-500/30',
  minor_gap:    'bg-yellow-500/10 text-yellow-400 border-yellow-500/30',
  moderate_gap: 'bg-orange-500/10 text-orange-400 border-orange-500/30',
  major_gap:    'bg-red-500/10    text-red-400    border-red-500/30',
}
const labelText: Record<DeltaLabel, string> = {
  aligned:      'Aligned',
  minor_gap:    'Minor Gap',
  moderate_gap: 'Moderate Gap',
  major_gap:    'Major Gap',
}

const colorClass = computed(() =>
  props.delta.label ? labelColor[props.delta.label] : 'bg-muted text-muted-foreground border-border'
)
const fmt = (v: number | null) => v !== null ? v.toFixed(1) : '—'
const deltaSign = computed(() => {
  if (props.delta.delta === null) return ''
  return props.delta.delta > 0 ? '+' : ''
})
</script>

<template>
  <div
    :class="['inline-flex flex-wrap items-center gap-2 px-3 py-1.5 rounded-lg border text-xs font-mono', colorClass]"
  >
    <!-- Agent consensus -->
    <span class="flex items-center gap-1">
      <span class="text-muted-foreground">🤖</span>
      <span class="font-bold">{{ fmt(delta.agent_consensus) }}</span>
      <span class="text-muted-foreground opacity-70">({{ delta.agent_review_cnt }})</span>
    </span>

    <span class="text-muted-foreground opacity-50">｜</span>

    <!-- Human consensus -->
    <span class="flex items-center gap-1">
      <span class="text-muted-foreground">👤</span>
      <span class="font-bold">{{ fmt(delta.human_consensus) }}</span>
      <span class="text-muted-foreground opacity-70">({{ delta.human_review_cnt }})</span>
    </span>

    <!-- Delta -->
    <template v-if="delta.delta !== null">
      <span class="text-muted-foreground opacity-50">｜</span>
      <span class="font-bold">Δ{{ deltaSign }}{{ fmt(delta.delta) }}</span>
      <span class="opacity-70">{{ labelText[delta.label] }}</span>
    </template>
  </div>
</template>
