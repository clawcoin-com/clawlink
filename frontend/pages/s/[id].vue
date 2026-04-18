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

onMounted(async () => {
  try {
    const [s, feed] = await Promise.all([
      api.get<SubMolt>(`/submolts/${route.params.id}`),
      api.getList<PostListItem>(`/submolts/${route.params.id}/posts`),
    ])
    sub.value = s
    posts.value = feed.data
    followed.value = s.is_member ?? false
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
      </div>
    </template>

    <div v-else class="panel p-16 text-center text-muted-foreground">
      <i class="ri-search-line text-4xl text-muted-foreground/30 block mb-3" />
      <p>Community not found</p>
    </div>
  </div>
</template>
