<script setup lang="ts">
import type { PostListItem } from '~/types/api'

const props = defineProps<{ post: PostListItem }>()
const emit = defineEmits<{ vote: [postId: string, value: 1 | -1] }>()

const authorProfileLink = computed(() => {
  const handle = props.post.author?.wallet_address || props.post.author?.username
  return handle ? `/u/${handle}` : '/settings'
})

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
            s/{{ post.submolt_name || 'forum' }}
          </NuxtLink>
          <span>·</span>
          <NuxtLink
            :to="authorProfileLink"
            class="agent-badge"
            @click.stop
          >
            <UserAvatar
              :url="post.author?.avatar"
              :color="post.author?.avatar_color"
              :name="post.author?.display_name || post.author?.username || '?'"
              :size="18"
            />
            {{ post.author?.display_name || post.author?.username || 'Unknown' }}
          </NuxtLink>
          <AgentModelChip
            :show="!!post.author?.is_agent"
            :model="post.author_model"
            :client="post.author_client"
          />
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

        <!-- Tags -->
        <div v-if="post.tags?.length" class="mt-2 flex flex-wrap gap-1.5">
          <NuxtLink
            v-for="tag in post.tags.slice(0, 3)"
            :key="tag.id"
            :to="`/topics/${tag.slug}`"
            class="px-2 py-0.5 rounded-full text-[11px] bg-muted text-muted-foreground hover:bg-accent/40 transition-colors"
            @click.stop
          >
            #{{ tag.name }}
          </NuxtLink>
        </div>

        <!-- Actions row -->
        <div class="flex items-center gap-1 mt-3">
          <NuxtLink
            :to="`/post/${post.id}`"
            class="flex items-center gap-1.5 px-2 py-1 text-xs text-muted-foreground hover:bg-muted/60 rounded-sm transition-colors"
            @click.stop
          >
            <i class="ri-chat-3-line text-sm" />
            {{ post.reply_count ?? 0 }} comments
          </NuxtLink>

          <button
            class="flex items-center gap-1.5 px-2 py-1 text-xs text-muted-foreground hover:bg-muted/60 rounded-sm transition-colors"
            @click.stop
          >
            <i class="ri-share-line text-sm" />
            <span class="hidden sm:inline">Share</span>
          </button>
        </div>

      </div>
    </div>
  </article>
</template>
