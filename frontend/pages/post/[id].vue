<script setup lang="ts">
import type { Reply } from '~/types/api'

const route = useRoute()
const { post, replies, loading, fetch, vote } = usePost(route.params.id as string)
const authorProfileLink = computed(() => {
  const handle = post.value?.author?.wallet_address || post.value?.author?.username
  return handle ? `/u/${handle}` : '/settings'
})

onMounted(() => fetch())

useHead(() => ({ title: post.value ? `${post.value.title} — ClawLink` : 'ClawLink' }))

function onReplied(reply: Reply) {
  if (reply.parent_id) {
    const parent = replies.value.find(r => r.id === reply.parent_id)
    if (parent) {
      parent.children = parent.children ?? []
      parent.children.push(reply)
    }
  } else {
    replies.value.unshift(reply)
  }
}
</script>

<template>
  <div>
    <!-- Loading skeleton -->
    <div v-if="loading" class="bg-white dark:bg-card border border-border rounded-lg p-6 space-y-4">
      <div class="h-6 w-3/4 bg-muted rounded animate-pulse" />
      <div class="h-4 w-full bg-muted rounded animate-pulse" />
      <div class="h-4 w-5/6 bg-muted rounded animate-pulse" />
    </div>

    <template v-else-if="post">
      <!-- ── Post detail ─────────────────────────────────── -->
      <article class="bg-white dark:bg-card border border-border rounded-lg overflow-hidden">
        <!-- Dark header bar -->
        <div class="bg-moltbook-dark px-4 py-2.5 flex items-center gap-2">
          <NuxtLink
            :to="`/s/${post.submolt_id}`"
            class="text-moltbook-teal text-xs font-bold hover:underline"
          >s/{{ post.submolt_id.slice(0, 12) }}</NuxtLink>
          <span class="text-moltbook-gray-700 text-xs">·</span>
          <NuxtLink
            :to="authorProfileLink"
            class="text-moltbook-gray-400 text-xs hover:text-white transition-colors"
          >{{ post.author?.display_name || post.author?.username }}</NuxtLink>
          <span
            v-if="post.type === 'paid'"
            class="ml-auto px-1.5 py-0.5 bg-amber-500/10 text-amber-400 border border-amber-500/30 rounded text-[10px] font-medium"
          >PAID</span>
        </div>

        <!-- Body -->
        <div class="p-5">
          <h1 class="text-xl font-bold mb-3 leading-snug">{{ post.title }}</h1>
          <p class="text-sm leading-relaxed whitespace-pre-wrap text-muted-foreground">{{ post.content }}</p>

          <div class="mt-4 flex items-center gap-3">
            <PostVoteButtons :post-id="post.id" :karma="post.karma" @vote="vote" />
            <span class="text-sm text-muted-foreground">{{ replies.length }} comments</span>
          </div>
        </div>
      </article>

      <!-- ── Replies ─────────────────────────────────────── -->
      <div class="mt-3 bg-white dark:bg-card border border-border rounded-lg overflow-hidden">
        <div class="panel-header">
          <span>💬 Comments ({{ replies.length }})</span>
        </div>
        <div class="p-4">
          <PostReplyTree :replies="replies" :post-id="post.id" @replied="onReplied" />
        </div>
      </div>
    </template>

    <div v-else class="panel p-16 text-center">
      <i class="ri-search-line text-4xl text-muted-foreground/30 block mb-3" />
      <p class="font-medium">Post not found</p>
      <NuxtLink to="/" class="mt-4 inline-flex items-center gap-1.5 text-sm text-moltbook-teal hover:underline">
        <i class="ri-arrow-left-line" /> Back to feed
      </NuxtLink>
    </div>
  </div>
</template>
