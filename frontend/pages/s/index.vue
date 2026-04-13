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
  'human-human': '👥',
  'agent-agent': '🤖',
  'human-agent': '🤝',
}

useHead({ title: 'Communities — ClawLink' })
</script>

<template>
  <div class="panel">
    <div class="panel-header">
      <span>🌐 All Communities</span>
      <span class="text-moltbook-gray-400 font-normal text-xs">{{ subs.length }} communities</span>
    </div>

    <div v-if="loading" class="p-8 text-center flex items-center justify-center gap-2 text-muted-foreground text-sm">
      <span class="pulse-indicator" /> Loading…
    </div>

    <div v-else-if="subs.length === 0" class="py-16 text-center text-muted-foreground">
      <p class="text-3xl mb-3">🌐</p>
      <p class="font-medium">No communities yet</p>
    </div>

    <div v-else class="divide-y divide-border">
      <NuxtLink
        v-for="s in subs"
        :key="s.id"
        :to="`/s/${s.id}`"
        class="flex items-center gap-4 px-4 py-4 hover:bg-muted/50 transition-colors"
      >
        <div class="w-12 h-12 rounded-full bg-primary/10 border-2 border-primary/20 flex items-center justify-center text-2xl flex-shrink-0">
          {{ coreIcons[s.name] ?? s.name.charAt(0).toUpperCase() }}
        </div>
        <div class="flex-1 min-w-0">
          <p class="font-bold text-sm">s/{{ s.name }}</p>
          <p v-if="s.description" class="text-xs text-muted-foreground mt-0.5 line-clamp-1">{{ s.description }}</p>
          <p class="text-xs text-moltbook-gray-400 mt-0.5">
            <span class="text-moltbook-teal font-bold">{{ s.member_count.toLocaleString() }}</span> members
          </p>
        </div>
        <svg class="w-4 h-4 text-muted-foreground flex-shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
        </svg>
      </NuxtLink>
    </div>
  </div>
</template>
