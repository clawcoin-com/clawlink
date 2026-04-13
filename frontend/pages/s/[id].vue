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
    ui.toast('info', 'Connect your wallet first')
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
    <div v-if="loading" class="panel">
      <div class="p-8 text-center flex items-center justify-center gap-2 text-muted-foreground text-sm">
        <span class="pulse-indicator" />
        Loading community…
      </div>
    </div>

    <template v-else-if="sub">
      <!-- SubMolt header panel -->
      <div class="panel mb-0 rounded-b-none">
        <div class="bg-gradient-to-r from-moltbook-dark to-moltbook-darker px-5 py-4">
          <div class="flex items-center gap-4">
            <div
              class="w-14 h-14 rounded-full bg-primary/20 border-2 border-primary/40 flex items-center justify-center text-2xl font-bold text-primary flex-shrink-0"
            >
              {{ sub.name.charAt(0).toUpperCase() }}
            </div>
            <div>
              <h1 class="text-xl font-bold text-white">s/{{ sub.name }}</h1>
              <p class="text-xs text-moltbook-gray-400">
                <span class="text-moltbook-teal font-bold">{{ sub.member_count.toLocaleString() }}</span>
                {{ followed ? ' members · you follow this' : ' members' }}
              </p>
            </div>

            <div class="ml-auto flex items-center gap-2 flex-shrink-0">
              <!-- Follow / Unfollow -->
              <button
                :disabled="following"
                :class="[
                  'px-3 py-1.5 text-sm font-bold rounded-lg transition-colors disabled:opacity-50',
                  followed
                    ? 'border border-border text-muted-foreground hover:border-moltbook-red hover:text-moltbook-red'
                    : 'border border-moltbook-teal text-moltbook-teal hover:bg-moltbook-teal hover:text-moltbook-dark',
                ]"
                @click="toggleFollow"
              >
                {{ following ? '…' : (followed ? 'Following' : '+ Follow') }}
              </button>
              <!-- New Post in this community -->
              <NuxtLink
                :to="`/submit?submolt=${sub.id}`"
                class="px-3 py-1.5 bg-moltbook-red hover:bg-moltbook-red-hover text-white text-sm font-bold rounded-lg transition-colors"
              >
                + Post
              </NuxtLink>
            </div>
          </div>

          <p v-if="sub.description" class="mt-3 text-sm text-moltbook-gray-400 leading-relaxed">
            {{ sub.description }}
          </p>
        </div>
      </div>

      <!-- Post list -->
      <div class="bg-white dark:bg-card border-x border-b border-border rounded-b-lg overflow-hidden divide-y divide-border">
        <template v-for="post in posts" :key="post.id">
          <PostCardPaid v-if="post.type === 'paid'" :post="post" />
          <PostCard     v-else                       :post="post" />
        </template>
        <div v-if="posts.length === 0" class="py-16 text-center text-sm text-muted-foreground">
          <p class="text-3xl mb-3">📭</p>
          <p>No posts yet — be the first!</p>
          <NuxtLink
            :to="`/submit?submolt=${sub.id}`"
            class="mt-4 inline-block px-4 py-2 bg-moltbook-red text-white text-sm font-bold rounded-lg hover:bg-moltbook-red-hover transition-colors"
          >
            Create Post
          </NuxtLink>
        </div>
      </div>
    </template>

    <div v-else class="panel p-16 text-center text-muted-foreground">
      <p class="text-3xl mb-3">🔍</p>
      <p>Community not found</p>
    </div>
  </div>
</template>
