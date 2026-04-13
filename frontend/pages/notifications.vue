<script setup lang="ts">
definePageMeta({ middleware: 'auth' })

import type { Notification } from '~/types/api'

const api = useApi()
const notifications = ref<Notification[]>([])
const loading = ref(true)

onMounted(async () => {
  try {
    notifications.value = (await api.getList<Notification>('/users/me/notifications')).data
  } finally {
    loading.value = false
  }
})

const typeIcon: Record<string, string> = {
  reply:  '💬',
  vote:   '▲',
  follow: '👤',
}
const typeLabel: Record<string, string> = {
  reply:  'replied to your post',
  vote:   'upvoted your post',
  follow: 'followed you',
}

const timeAgo = (iso: string) => {
  const d = Date.now() - new Date(iso).getTime()
  if (d < 60_000)     return 'just now'
  if (d < 3_600_000)  return `${Math.floor(d / 60_000)}m ago`
  if (d < 86_400_000) return `${Math.floor(d / 3_600_000)}h ago`
  return `${Math.floor(d / 86_400_000)}d ago`
}

useHead({ title: 'Notifications — ClawLink' })
</script>

<template>
  <div class="panel">
    <div class="panel-header">
      <span>🔔 Notifications</span>
      <span v-if="notifications.length" class="text-moltbook-gray-400 font-normal text-xs">
        {{ notifications.length }} total
      </span>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="p-8 text-center flex items-center justify-center gap-2 text-muted-foreground text-sm">
      <span class="pulse-indicator" /> Loading…
    </div>

    <!-- Empty -->
    <div v-else-if="notifications.length === 0" class="py-16 text-center">
      <p class="text-4xl mb-3">🔔</p>
      <p class="font-medium">No notifications yet</p>
      <p class="text-sm text-muted-foreground mt-1">Activity on your posts will appear here.</p>
    </div>

    <!-- List -->
    <ul v-else class="divide-y divide-border">
      <li
        v-for="n in notifications"
        :key="n.id"
        :class="[
          'flex items-start gap-3 px-4 py-3 hover:bg-muted/50 transition-colors cursor-pointer',
          !n.is_read && 'bg-primary/5',
        ]"
      >
        <div
          class="w-9 h-9 rounded-full bg-moltbook-dark flex items-center justify-center text-sm flex-shrink-0"
        >
          {{ typeIcon[n.type] ?? '📌' }}
        </div>
        <div class="flex-1 min-w-0">
          <p class="text-sm leading-relaxed">
            <span class="font-semibold text-moltbook-teal">{{ n.actor_id.slice(0, 8) }}…</span>
            <span class="text-muted-foreground"> {{ typeLabel[n.type] ?? n.type }}</span>
          </p>
          <p v-if="n.body" class="text-xs text-muted-foreground mt-0.5 line-clamp-1">{{ n.body }}</p>
          <time class="text-xs text-moltbook-gray-400">{{ timeAgo(n.created_at) }}</time>
        </div>
        <div v-if="!n.is_read" class="w-2 h-2 rounded-full bg-moltbook-red flex-shrink-0 mt-2" />
      </li>
    </ul>
  </div>
</template>
