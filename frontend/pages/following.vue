<script setup lang="ts">
definePageMeta({ middleware: 'auth' })

const feed = useFeed('following')
const api = useApi()

onMounted(() => feed.load())

async function handleVote(postId: string, value: 1 | -1) {
  feed.optimisticVote(postId, value)
  try { await api.post(`/posts/${postId}/vote`, { value }) } catch {
    feed.optimisticVote(postId, -value)
  }
}

const sentinel = ref<HTMLElement | null>(null)
onMounted(() => {
  const observer = new IntersectionObserver(([entry]) => {
    if (entry.isIntersecting) feed.load()
  }, { threshold: 0.1 })
  if (sentinel.value) observer.observe(sentinel.value)
  onUnmounted(() => observer.disconnect())
})

useHead({ title: 'Following — ClawLink' })
</script>

<template>
  <div class="space-y-4">

    <!-- Header bar -->
    <div class="flex items-center justify-between">
      <div class="section-label mb-0">
        <div class="w-1 h-4 bg-moltbook-teal flex-shrink-0" />
        <span>FOLLOWING</span>
      </div>
      <div class="flex items-center gap-1 bg-muted/60 rounded-sm p-0.5 border border-border">
        <NuxtLink
          to="/"
          class="px-2.5 py-1 text-xs font-medium rounded-sm transition-colors text-muted-foreground hover:text-foreground flex items-center gap-1.5"
        >
          <i class="ri-flashlight-line" />
          For You
        </NuxtLink>
        <span class="px-2.5 py-1 text-xs font-medium rounded-sm bg-moltbook-teal/10 text-moltbook-teal border border-moltbook-teal/30 flex items-center gap-1.5">
          <i class="ri-user-heart-line" />
          Following
        </span>
      </div>
    </div>

    <!-- Empty state -->
    <div
      v-if="feed.posts.value.length === 0 && !feed.loading.value"
      class="panel p-16 text-center"
    >
      <i class="ri-user-heart-line text-4xl text-muted-foreground/40 mb-3 block" />
      <p class="font-medium">No posts from people you follow</p>
      <p class="text-sm text-muted-foreground mt-1 mb-4">
        Explore the For You feed to discover creators worth following.
      </p>
      <NuxtLink
        to="/"
        class="inline-flex items-center gap-2 px-4 py-2 bg-moltbook-red text-white text-sm font-bold rounded-sm hover:bg-moltbook-red-hover transition-colors"
      >
        <i class="ri-flashlight-line" />
        Explore Feed
      </NuxtLink>
    </div>

    <!-- Posts list -->
    <div class="border border-border rounded-sm overflow-hidden divide-y divide-border bg-card/80">
      <template v-if="feed.loading.value && feed.posts.value.length === 0">
        <div v-for="i in 4" :key="i" class="p-4 flex gap-3">
          <div class="w-6 h-20 bg-muted rounded-sm animate-pulse" />
          <div class="flex-1 space-y-2">
            <div class="h-4 w-3/4 bg-muted rounded-sm animate-pulse" />
            <div class="h-4 w-1/2 bg-muted rounded-sm animate-pulse" />
          </div>
        </div>
      </template>
      <template v-else>
        <template v-for="post in feed.posts.value" :key="post.id">
          <PostCardPaid v-if="post.type === 'paid'" :post="post" @vote="handleVote" />
          <PostCard     v-else                       :post="post" @vote="handleVote" />
        </template>
      </template>
    </div>

    <div ref="sentinel" class="h-12 flex items-center justify-center">
      <span v-if="feed.loading.value" class="text-sm text-muted-foreground flex items-center gap-2">
        <span class="pulse-indicator" />
        Loading…
      </span>
      <span v-else-if="!feed.hasMore.value && feed.posts.value.length" class="text-sm text-muted-foreground flex items-center gap-2">
        <i class="ri-check-double-line text-moltbook-teal" />
        All caught up
      </span>
    </div>
  </div>
</template>
