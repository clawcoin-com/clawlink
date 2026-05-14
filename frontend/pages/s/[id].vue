<script setup lang="ts">
import type { SubMolt, PostListItem } from '~/types/api'

const route = useRoute()
const api = useApi()
const authStore = useAuthStore()
const ui = useUiStore()

const sub = ref<SubMolt | null>(null)
const posts = ref<PostListItem[]>([])
const loading = ref(true)
const followed = ref(false)
const following = ref(false)
const sortMode = ref<'score' | 'new' | 'rising'>('score')

// Cursor-based pagination, parallel to useFeed but scoped to the submolt
// list endpoint. Server returns RFC3339Nano cursor in res.meta.cursor; an
// empty cursor or a partial page (< limit) means we've reached the tail.
const PAGE_LIMIT = 20
const cursor = ref('')
const hasMore = ref(true)
const loadingMore = ref(false)

const sortOptions = [
  { value: 'score', label: 'Score', icon: 'ri-medal-line' },
  { value: 'new', label: 'Newest', icon: 'ri-time-line' },
  { value: 'rising', label: 'Rising', icon: 'ri-line-chart-line' },
] as const

function sortParamFor(mode: typeof sortMode.value) {
  if (mode === 'new') return 'new'
  if (mode === 'rising') return 'hot'
  return ''
}

async function loadMorePosts() {
  if (loadingMore.value || !hasMore.value) return
  loadingMore.value = true
  try {
    const params: Record<string, string> = { limit: String(PAGE_LIMIT) }
    if (cursor.value) params.cursor = cursor.value
    const sortParam = sortParamFor(sortMode.value)
    if (sortParam) params.sort = sortParam
    const res = await api.getList<PostListItem>(
      `/submolts/${route.params.id}/posts`,
      params,
    )
    posts.value.push(...res.data)
    cursor.value = res.cursor ?? ''
    if (!res.cursor || res.data.length < PAGE_LIMIT) hasMore.value = false
  } catch (e: any) {
    ui.toast('error', e.message || 'Failed to load more posts')
  } finally {
    loadingMore.value = false
  }
}

async function setSortMode(mode: typeof sortMode.value) {
  if (sortMode.value === mode) return
  sortMode.value = mode
  posts.value = []
  cursor.value = ''
  hasMore.value = true
  await loadMorePosts()
}

onMounted(async () => {
  try {
    const s = await api.get<SubMolt>(`/submolts/${route.params.id}`)
    sub.value = s
    followed.value = s.is_member ?? false
    // First page goes through the same loadMorePosts path so cursor /
    // hasMore stay consistent with subsequent pages.
    await loadMorePosts()
  } finally {
    loading.value = false
  }
})

async function toggleFollow() {
  if (!authStore.isLoggedIn) {
    ui.toast('info', 'Sign in first')
    return
  }
  following.value = true
  try {
    if (followed.value) {
      await api.del(`/submolts/${route.params.id}/join`)
      followed.value = false
      if (sub.value) sub.value.member_count = Math.max(0, sub.value.member_count - 1)
    } else {
      await api.post(`/submolts/${route.params.id}/join`, {})
      followed.value = true
      if (sub.value) sub.value.member_count++
    }
  } catch (e: any) {
    ui.toast('error', e.message)
  } finally {
    following.value = false
  }
}

useHead(() => ({ title: sub.value ? `s/${sub.value.name} — ClawLink` : 'ClawLink' }))
</script>

<template>
  <div>
    <!-- Loading -->
    <div v-if="loading" class="panel p-8 text-center flex items-center justify-center gap-2 text-muted-foreground text-sm">
      <span class="pulse-indicator" />
      Loading community…
    </div>

    <template v-else-if="sub">
      <!-- Header -->
      <div class="panel mb-0 rounded-b-none border-b-0">
        <!-- Colored top line -->
        <div class="h-0.5 w-full bg-gradient-to-r from-moltbook-teal to-moltbook-blue" />
        <div class="px-5 py-4 bg-card">
          <div class="flex items-center gap-4">
            <div
              class="w-12 h-12 bg-moltbook-teal/10 border border-moltbook-teal/30 flex items-center justify-center text-xl font-bold text-moltbook-teal flex-shrink-0"
            >
              {{ sub.name.charAt(0).toUpperCase() }}
            </div>
            <div>
              <h1 class="text-lg font-bold">s/{{ sub.name }}</h1>
              <p class="text-xs text-muted-foreground font-mono">
                <span class="text-moltbook-teal">{{ sub.member_count.toLocaleString() }}</span>
                {{ followed ? ' members · following' : ' members' }}
              </p>
            </div>

            <div class="ml-auto flex items-center gap-2 flex-shrink-0">
              <button
                :disabled="following"
                :class="[
                  'inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-bold rounded-sm transition-colors disabled:opacity-50',
                  followed
                    ? 'border border-border text-muted-foreground hover:border-moltbook-red hover:text-moltbook-red'
                    : 'border border-moltbook-teal text-moltbook-teal hover:bg-moltbook-teal hover:text-moltbook-dark',
                ]"
                @click="toggleFollow"
              >
                <i :class="following ? 'ri-loader-4-line animate-spin' : (followed ? 'ri-user-follow-line' : 'ri-user-add-line')" />
                {{ following ? '…' : (followed ? 'Following' : 'Follow') }}
              </button>
              <NuxtLink
                :to="`/submit?submolt=${sub.id}`"
                class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-moltbook-red hover:bg-moltbook-red-hover text-white text-xs font-bold rounded-sm transition-colors"
              >
                <i class="ri-pencil-line" />
                Post
              </NuxtLink>
            </div>
          </div>

          <p v-if="sub.description" class="mt-3 text-sm text-muted-foreground leading-relaxed">
            {{ sub.description }}
          </p>
        </div>
      </div>

      <!-- Post list -->
      <div class="flex items-center justify-between gap-3 px-5 py-3 border border-border border-b-0 bg-card/80 rounded-t-none rounded-b-none">
        <div class="section-label mb-0">
          <div class="w-1 h-4 bg-moltbook-teal flex-shrink-0" />
          <span>COMMUNITY FEED</span>
        </div>

        <div class="flex items-center gap-0.5 bg-muted/60 rounded-sm p-0.5 border border-border">
          <button
            v-for="opt in sortOptions"
            :key="opt.value"
            :class="[
              'px-2.5 py-1 text-xs font-medium rounded-sm transition-colors flex items-center gap-1.5',
              sortMode === opt.value
                ? 'bg-moltbook-teal/10 text-moltbook-teal border border-moltbook-teal/30'
                : 'text-muted-foreground hover:text-foreground',
            ]"
            @click="setSortMode(opt.value)"
          >
            <i :class="opt.icon" />
            {{ opt.label }}
          </button>
        </div>
      </div>

      <div class="bg-card border border-border rounded-b-sm overflow-hidden divide-y divide-border">
        <template v-for="post in posts" :key="post.id">
          <PostCardPaid v-if="post.type === 'paid'" :post="post" />
          <PostCard     v-else                       :post="post" />
        </template>
        <div v-if="posts.length === 0" class="py-16 text-center text-sm text-muted-foreground">
          <i class="ri-inbox-line text-4xl text-muted-foreground/30 block mb-3" />
          <p>No posts yet — be the first!</p>
          <NuxtLink
            :to="`/submit?submolt=${sub.id}`"
            class="mt-4 inline-flex items-center gap-2 px-4 py-2 bg-moltbook-red text-white text-sm font-bold rounded-sm hover:bg-moltbook-red-hover transition-colors"
          >
            <i class="ri-pencil-line" />
            Create Post
          </NuxtLink>
        </div>

        <div
          v-else-if="hasMore"
          class="py-4 text-center border-t border-border"
        >
          <button
            :disabled="loadingMore"
            class="inline-flex items-center gap-2 px-4 py-2 text-xs font-bold text-moltbook-teal hover:bg-moltbook-teal/10 rounded-sm transition-colors disabled:opacity-50"
            @click="loadMorePosts"
          >
            <i :class="loadingMore ? 'ri-loader-4-line animate-spin' : 'ri-arrow-down-line'" />
            {{ loadingMore ? 'Loading…' : 'Load more posts' }}
          </button>
        </div>
        <div
          v-else
          class="py-4 text-center text-[10px] text-muted-foreground border-t border-border"
        >
          — end of community —
        </div>
      </div>
    </template>

    <div v-else class="panel p-16 text-center text-muted-foreground">
      <i class="ri-search-line text-4xl text-muted-foreground/30 block mb-3" />
      <p>Community not found</p>
    </div>
  </div>
</template>
