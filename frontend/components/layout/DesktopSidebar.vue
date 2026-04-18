<script setup lang="ts">
import type { SubMolt } from '~/types/api'

const api = useApi()
const route = useRoute()
const subs = ref<SubMolt[]>([])

onMounted(async () => {
  try { subs.value = await api.get<SubMolt[]>('/submolts') } catch {}
})

const coreIcons: Record<string, string> = {
  'human-human': 'ri-team-line',
  'agent-agent': 'ri-robot-line',
  'human-agent': 'ri-shake-hands-line',
}

const navLinks = [
  { to: '/',          label: 'Home',        icon: 'ri-home-5-line' },
  { to: '/following', label: 'Following',   icon: 'ri-user-heart-line' },
  { to: '/s',         label: 'Communities', icon: 'ri-layout-grid-line' },
  { to: '/docs',      label: 'Docs',        icon: 'ri-file-text-line' },
  { to: '/settings',  label: 'Settings',    icon: 'ri-settings-3-line' },
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
            'flex items-center gap-3 px-3 py-2 rounded-sm text-sm transition-colors',
            isActive(link.to)
              ? 'bg-moltbook-teal/10 text-moltbook-teal border-l-2 border-moltbook-teal pl-[10px]'
              : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground',
          ]"
        >
          <i :class="[link.icon, 'text-base w-4 text-center flex-shrink-0']" />
          {{ link.label }}
        </NuxtLink>
      </div>

      <!-- Communities -->
      <div v-if="subs.length">
        <div class="px-3 mb-2 flex items-center gap-2">
          <div class="w-0.5 h-3 bg-moltbook-teal" />
          <h3 class="text-[10px] font-semibold text-muted-foreground uppercase tracking-[0.15em]">
            Communities
          </h3>
        </div>
        <div class="space-y-0.5">
          <NuxtLink
            v-for="s in subs"
            :key="s.id"
            :to="`/s/${s.id}`"
            :class="[
              'flex items-center gap-3 px-3 py-2 rounded-sm text-sm transition-colors',
              route.path === `/s/${s.id}`
                ? 'bg-moltbook-teal/10 text-moltbook-teal border-l-2 border-moltbook-teal pl-[10px]'
                : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground',
            ]"
          >
            <i
              v-if="coreIcons[s.name]"
              :class="[coreIcons[s.name], 'w-4 text-center flex-shrink-0 text-base']"
            />
            <span
              v-else
              class="w-4 h-4 bg-primary/10 text-primary flex items-center justify-center text-[10px] font-bold flex-shrink-0"
            >{{ s.name.charAt(0).toUpperCase() }}</span>
            <span class="truncate">s/{{ s.name }}</span>
          </NuxtLink>
        </div>
      </div>

      <!-- New Post -->
      <div class="px-2">
        <NuxtLink
          to="/submit"
          class="flex items-center justify-center gap-2 w-full py-2 bg-moltbook-red hover:bg-moltbook-red-hover text-white text-sm font-bold rounded-sm transition-colors"
        >
          <i class="ri-pencil-line" />
          New Post
        </NuxtLink>
      </div>

    </nav>
  </aside>
</template>
