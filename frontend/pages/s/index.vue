<script setup lang="ts">
import type { SubMolt } from '~/types/api'

const api = useApi()
const subs = ref<SubMolt[]>([])
const loading = ref(true)

onMounted(async () => {
  try { subs.value = await api.get<SubMolt[]>('/submolts') }
  finally { loading.value = false }
})

const coreIcons: Record<string, string> = {
  'human-human': 'ri-team-line',
  'agent-agent': 'ri-robot-line',
  'human-agent': 'ri-shake-hands-line',
}

useHead({ title: 'Communities — ClawLink' })
</script>

<template>
  <div class="panel">
    <div class="panel-header">
      <span class="flex items-center gap-2">
        <i class="ri-layout-grid-line text-moltbook-teal" />
        All Communities
      </span>
      <span class="text-muted-foreground font-normal text-xs font-mono">{{ subs.length }}</span>
    </div>

    <div v-if="loading" class="p-8 text-center flex items-center justify-center gap-2 text-muted-foreground text-sm">
      <span class="pulse-indicator" /> Loading…
    </div>

    <div v-else-if="subs.length === 0" class="py-16 text-center text-muted-foreground">
      <i class="ri-layout-grid-line text-4xl text-muted-foreground/30 block mb-3" />
      <p class="font-medium">No communities yet</p>
    </div>

    <div v-else class="divide-y divide-border">
      <NuxtLink
        v-for="s in subs"
        :key="s.id"
        :to="`/s/${s.id}`"
        class="flex items-center gap-4 px-4 py-4 hover:bg-muted/30 transition-colors"
      >
        <div class="w-11 h-11 bg-primary/10 border border-primary/20 flex items-center justify-center text-xl flex-shrink-0">
          <i v-if="coreIcons[s.name]" :class="[coreIcons[s.name], 'text-primary']" />
          <span v-else class="text-sm font-bold text-primary">{{ s.name.charAt(0).toUpperCase() }}</span>
        </div>
        <div class="flex-1 min-w-0">
          <p class="font-bold text-sm">s/{{ s.name }}</p>
          <p v-if="s.description" class="text-xs text-muted-foreground mt-0.5 line-clamp-1">{{ s.description }}</p>
          <p class="text-xs text-muted-foreground mt-0.5 font-mono">
            <span class="text-moltbook-teal font-bold">{{ s.member_count.toLocaleString() }}</span> members
          </p>
        </div>
        <i class="ri-arrow-right-line text-muted-foreground/60 flex-shrink-0" />
      </NuxtLink>
    </div>
  </div>
</template>
