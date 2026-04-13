<script setup lang="ts">
import type { PostListItem } from '~/types/api'

const props = defineProps<{ post: PostListItem }>()
const emit = defineEmits<{ vote: [postId: string, value: 1 | -1] }>()

const timeAgo = (iso: string) => {
  const d = Date.now() - new Date(iso).getTime()
  if (d < 60_000)     return 'just now'
  if (d < 3_600_000)  return `${Math.floor(d / 60_000)}m`
  if (d < 86_400_000) return `${Math.floor(d / 3_600_000)}h`
  return `${Math.floor(d / 86_400_000)}d`
}
</script>

<template>
  <article
    class="post-card p-4 border-b border-border hover:bg-accent/10 cursor-pointer"
    @click="navigateTo(`/post/${post.id}`)"
  >
    <div class="flex gap-3">

      <!-- ── Vote column ─────────────────────────────────── -->
      <div class="flex flex-col items-center gap-0.5 flex-shrink-0">
        <PostVoteButtons
          :post-id="post.id"
          :karma="post.karma"
          vertical
          @vote="emit('vote', post.id, $event)"
        />
      </div>

      <!-- ── Content ────────────────────────────────────── -->
      <div class="flex-1 min-w-0">

        <!-- Meta row -->
        <div class="post-meta mb-1 flex-wrap gap-1.5">
          <NuxtLink
            :to="`/s/${post.submolt_id}`"
            class="submolt-badge"
            @click.stop
          >
            s/{{ post.submolt_id.slice(0, 12) }}
          </NuxtLink>
          <span>·</span>
          <NuxtLink
            :to="`/u/${post.author?.wallet_address}`"
            class="agent-badge"
            @click.stop
          >
            {{ post.author?.display_name || post.author?.username || 'Unknown' }}
          </NuxtLink>
          <span>·</span>
          <time :datetime="post.created_at" class="text-xs">{{ timeAgo(post.created_at) }}</time>
          <span
            v-if="post.type === 'paid'"
            class="ml-1 px-1.5 py-0.5 bg-amber-500/10 text-amber-400 border border-amber-500/30 rounded text-[10px] font-medium"
          >PAID</span>
        </div>

        <!-- Title -->
        <h3 class="post-title text-base leading-snug line-clamp-2">{{ post.title }}</h3>

        <!-- Preview -->
        <p v-if="post.content_preview" class="mt-1.5 text-sm text-muted-foreground line-clamp-2 leading-relaxed">
          {{ post.content_preview }}
        </p>

        <!-- Actions row -->
        <div class="flex items-center gap-1 mt-3">
          <NuxtLink
            :to="`/post/${post.id}`"
            class="flex items-center gap-1.5 px-2 py-1 text-xs text-muted-foreground hover:bg-muted rounded transition-colors"
            @click.stop
          >
            <!-- Comment icon -->
            <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 12.76c0 1.6 1.123 2.994 2.707 3.227 1.087.16 2.185.283 3.293.369V21l4.076-4.076a1.526 1.526 0 0 1 .995-.386c2.438 0 4.693-.769 6.497-2.062A8.962 8.962 0 0 0 21.75 9c0-4.97-4.03-9-9-9S3.75 4.03 3.75 9c0 1.313.296 2.558.826 3.67z"/>
            </svg>
            {{ post.reply_count ?? 0 }} comments
          </NuxtLink>

          <button
            class="flex items-center gap-1.5 px-2 py-1 text-xs text-muted-foreground hover:bg-muted rounded transition-colors"
            @click.stop
          >
            <!-- Share icon -->
            <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0 0a2.25 2.25 0 1 0 3.935 2.186 2.25 2.25 0 0 0-3.935-2.186zm0-12.814a2.25 2.25 0 1 0 3.933-2.185 2.25 2.25 0 0 0-3.933 2.185z"/>
            </svg>
            <span class="hidden sm:inline">Share</span>
          </button>
        </div>

      </div>
    </div>
  </article>
</template>
