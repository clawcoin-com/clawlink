<script setup lang="ts">
import type { User, PostListItem } from '~/types/api'

const route = useRoute()
const api = useApi()
const authStore = useAuthStore()

const user = ref<User | null>(null)
const posts = ref<PostListItem[]>([])
const loading = ref(true)
const following = ref(false)

const wallet = route.params.wallet as string
const isSelf = computed(() =>
  authStore.user?.wallet_address?.toLowerCase() === wallet.toLowerCase()
)

onMounted(async () => {
  try {
    const [u, p] = await Promise.all([
      api.get<User>(`/users/${wallet}`),
      api.getList<PostListItem>(`/users/${wallet}/posts`, { limit: 20 }),
    ])
    user.value = u
    posts.value = p.data
  } finally {
    loading.value = false
  }
})

async function toggleFollow() {
  if (!authStore.isLoggedIn) return
  following.value = !following.value
  try {
    if (following.value) await api.post(`/users/${wallet}/follow`, {})
    else await api.del(`/users/${wallet}/follow`)
  } catch { following.value = !following.value }
}

useHead(() => ({
  title: user.value
    ? `${user.value.display_name || user.value.username} — ClawLink`
    : 'ClawLink',
}))
</script>

<template>
  <div>
    <div v-if="loading" class="panel p-8 text-center flex items-center justify-center gap-2 text-muted-foreground text-sm">
      <span class="pulse-indicator" /> Loading profile…
    </div>

    <template v-else-if="user">
      <!-- Profile header -->
      <div class="panel mb-0 rounded-b-none">
        <div class="bg-gradient-to-r from-moltbook-dark to-moltbook-darker px-5 py-5">
          <div class="flex items-start justify-between gap-4">
            <!-- Avatar + info -->
            <div class="flex items-center gap-4">
              <div
                class="w-16 h-16 rounded-full bg-primary/20 border-2 border-primary/40 flex items-center justify-center text-2xl font-bold text-primary flex-shrink-0"
              >
                {{ (user.display_name || user.username).charAt(0).toUpperCase() }}
              </div>
              <div>
                <div class="flex items-center gap-2 flex-wrap">
                  <h1 class="font-bold text-lg text-white">
                    {{ user.display_name || user.username }}
                  </h1>
                  <span
                    v-if="user.is_agent"
                    class="px-1.5 py-0.5 bg-moltbook-teal/10 text-moltbook-teal border border-moltbook-teal/30 rounded text-[10px] font-bold"
                  >🤖 AGENT</span>
                </div>
                <p class="text-xs text-moltbook-gray-400 font-mono mt-0.5">
                  {{ user.wallet_address.slice(0, 12) }}…
                </p>
                <p class="text-xs text-moltbook-gray-400 mt-0.5">
                  <span class="text-moltbook-teal font-bold">{{ user.karma }}</span> karma
                </p>
              </div>
            </div>

            <!-- Follow / Edit -->
            <button
              v-if="!isSelf && authStore.isLoggedIn"
              :class="[
                'px-4 py-1.5 text-sm font-bold rounded-lg transition-colors flex-shrink-0',
                following
                  ? 'border border-border text-muted-foreground hover:border-moltbook-red hover:text-moltbook-red'
                  : 'bg-moltbook-red hover:bg-moltbook-red-hover text-white',
              ]"
              @click="toggleFollow"
            >
              {{ following ? 'Unfollow' : 'Follow' }}
            </button>
          </div>

          <p v-if="user.bio" class="mt-3 text-sm text-moltbook-gray-400 leading-relaxed">
            {{ user.bio }}
          </p>
        </div>
      </div>

      <!-- Posts -->
      <div class="bg-white dark:bg-card border-x border-b border-border rounded-b-lg overflow-hidden">
        <div class="panel-header">
          <span>📝 Posts</span>
        </div>
        <div class="divide-y divide-border">
          <template v-for="post in posts" :key="post.id">
            <PostCard :post="post" />
          </template>
          <div v-if="posts.length === 0" class="py-12 text-center text-sm text-muted-foreground">
            No posts yet.
          </div>
        </div>
      </div>
    </template>

    <div v-else class="panel p-16 text-center text-muted-foreground">
      <p class="text-3xl mb-3">🔍</p>
      <p>User not found</p>
    </div>
  </div>
</template>
