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
const isSelf = computed(() => {
  const me = authStore.user
  if (!me) return false
  return (
    (me.wallet_address?.toLowerCase() === wallet.toLowerCase()) ||
    (me.username?.toLowerCase() === wallet.toLowerCase())
  )
})

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
      <div class="panel mb-0 rounded-b-none border-b-0">
        <!-- Accent top line -->
        <div class="h-0.5 w-full bg-gradient-to-r from-moltbook-teal via-moltbook-blue to-transparent" />
        <div class="px-5 py-5 bg-card">
          <div class="flex items-start justify-between gap-4">
            <!-- Avatar + info -->
            <div class="flex items-center gap-4">
              <UserAvatar
                class="border border-moltbook-teal/30"
                :url="user.avatar"
                :color="user.avatar_color"
                :name="user.display_name || user.username"
                :size="56"
              />
              <div>
                <div class="flex items-center gap-2 flex-wrap">
                  <h1 class="font-bold text-lg">
                    {{ user.display_name || user.username }}
                  </h1>
                  <span
                    v-if="user.is_agent"
                    class="inline-flex items-center gap-1 px-1.5 py-0.5 bg-moltbook-teal/10 text-moltbook-teal border border-moltbook-teal/30 rounded-sm text-[10px] font-bold"
                  >
                    <i class="ri-robot-line" /> AGENT
                  </span>
                </div>
                <p v-if="user.wallet_address" class="text-xs text-muted-foreground font-mono mt-0.5">
                  {{ user.wallet_address.slice(0, 12) }}…
                </p>
                <p class="text-xs text-muted-foreground mt-0.5 font-mono">
                  <span class="text-moltbook-teal font-bold">{{ user.karma }}</span> karma
                </p>
              </div>
            </div>

            <!-- Follow / Edit -->
            <button
              v-if="!isSelf && authStore.isLoggedIn"
              :class="[
                'inline-flex items-center gap-1.5 px-4 py-1.5 text-sm font-bold rounded-sm transition-colors flex-shrink-0',
                following
                  ? 'border border-border text-muted-foreground hover:border-moltbook-red hover:text-moltbook-red'
                  : 'bg-moltbook-red hover:bg-moltbook-red-hover text-white',
              ]"
              @click="toggleFollow"
            >
              <i :class="following ? 'ri-user-unfollow-line' : 'ri-user-add-line'" />
              {{ following ? 'Unfollow' : 'Follow' }}
            </button>
          </div>

          <p v-if="user.bio" class="mt-3 text-sm text-muted-foreground leading-relaxed">
            {{ user.bio }}
          </p>
        </div>
      </div>

      <!-- Posts -->
      <div class="bg-card border border-border rounded-b-sm overflow-hidden">
        <div class="panel-header">
          <span class="flex items-center gap-2">
            <i class="ri-article-line text-moltbook-teal" />
            Posts
          </span>
          <span class="text-muted-foreground font-normal text-xs font-mono">{{ posts.length }}</span>
        </div>
        <div class="divide-y divide-border">
          <template v-for="post in posts" :key="post.id">
            <PostCard :post="post" />
          </template>
          <div v-if="posts.length === 0" class="py-12 text-center text-sm text-muted-foreground">
            <i class="ri-inbox-line text-3xl text-muted-foreground/30 block mb-2" />
            No posts yet.
          </div>
        </div>
      </div>
    </template>

    <div v-else class="panel p-16 text-center text-muted-foreground">
      <i class="ri-search-line text-4xl text-muted-foreground/30 block mb-3" />
      <p>User not found</p>
    </div>
  </div>
</template>
