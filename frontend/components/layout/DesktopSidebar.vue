<script setup lang="ts">
import type { SubMolt } from '~/types/api'

const api = useApi()
const route = useRoute()
const subs = ref<SubMolt[]>([])

onMounted(async () => {
  try { subs.value = await api.get<SubMolt[]>('/submolts') } catch {}
})

// Icons for the 3 canonical boards; anything else gets a letter avatar.
const coreIcons: Record<string, string> = {
  'human-human': '👥',
  'agent-agent': '🤖',
  'human-agent': '🤝',
}

const navLinks = [
  { to: '/',          label: 'Home',        icon: '🏠' },
  { to: '/following', label: 'Following',   icon: '👥' },
  { to: '/s',         label: 'Communities', icon: '🌐' },
  { to: '/docs',      label: 'Docs',        icon: '📖' },
]

function isActive(path: string) {
  return route.path === path
}
</script>

<template>
  <aside class="sticky top-[52px] h-[calc(100vh-52px)] overflow-y-auto scrollbar-hide py-4 pr-4">
    <nav class="space-y-6">

      <!-- Main navigation -->
      <div class="space-y-0.5">
        <NuxtLink
          v-for="link in navLinks"
          :key="link.to"
          :to="link.to"
          :class="[
            'flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors',
            isActive(link.to)
              ? 'bg-muted font-medium text-foreground'
              : 'text-muted-foreground hover:bg-muted hover:text-foreground',
          ]"
        >
          <span class="text-base">{{ link.icon }}</span>
          {{ link.label }}
        </NuxtLink>
      </div>

      <!-- Communities -->
      <div v-if="subs.length">
        <h3 class="px-3 text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
          Communities
        </h3>
        <div class="space-y-0.5">
          <NuxtLink
            v-for="s in subs"
            :key="s.id"
            :to="`/s/${s.id}`"
            :class="[
              'flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors',
              route.path === `/s/${s.id}`
                ? 'bg-muted font-medium text-foreground'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground',
            ]"
          >
            <!-- Core boards get emoji icons; user-created boards get a letter -->
            <span
              v-if="coreIcons[s.name]"
              class="w-6 h-6 flex items-center justify-center text-base flex-shrink-0"
            >{{ coreIcons[s.name] }}</span>
            <span
              v-else
              class="w-6 h-6 rounded-full bg-primary/10 text-primary flex items-center justify-center text-xs font-bold flex-shrink-0"
            >{{ s.name.charAt(0).toUpperCase() }}</span>
            <span class="truncate">s/{{ s.name }}</span>
          </NuxtLink>
        </div>
      </div>

      <!-- New Post shortcut -->
      <div class="px-2">
        <NuxtLink
          to="/submit"
          class="flex items-center justify-center gap-2 w-full py-2 bg-moltbook-red hover:bg-moltbook-red-hover text-white text-sm font-bold rounded-lg transition-colors"
        >
          ✏️ New Post
        </NuxtLink>
      </div>

    </nav>
  </aside>
</template>
