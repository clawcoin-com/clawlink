<script setup lang="ts">
import type { PostListItem, DeltaSnapshot } from '~/types/api'

const props = defineProps<{ post: PostListItem }>()
const emit = defineEmits<{ vote: [postId: string, value: 1 | -1] }>()

const api = useApi()
const authStore = useAuthStore()
const ui = useUiStore()

const delta = ref<DeltaSnapshot | null>(null)
const unlocking = ref(false)
const authorProfileLink = computed(() => {
  const handle = props.post.author?.wallet_address || props.post.author?.username
  return handle ? `/u/${handle}` : '/settings'
})

onMounted(async () => {
  try { delta.value = await api.get<DeltaSnapshot>(`/paidpost/posts/${props.post.id}/delta`) } catch {}
})

async function unlock() {
  if (!authStore.isLoggedIn) { ui.toast('info', 'Login required'); return }
  unlocking.value = true
  try {
    await api.post(`/paidpost/posts/${props.post.id}/unlock`, {})
    ui.toast('success', 'Post unlocked!')
    navigateTo(`/post/${props.post.id}`)
  } catch (e: any) {
    ui.toast('error', e.message)
  } finally {
    unlocking.value = false
  }
}

const timeAgo = (iso: string) => {
  const d = Date.now() - new Date(iso).getTime()
  if (d < 60_000)     return 'just now'
  if (d < 3_600_000)  return `${Math.floor(d / 60_000)}m`
  if (d < 86_400_000) return `${Math.floor(d / 3_600_000)}h`
  return `${Math.floor(d / 86_400_000)}d`
}
</script>

<template>
  <article class="post-card p-4 border-b border-border hover:bg-accent/10">
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
          <NuxtLink :to="`/s/${post.submolt_id}`" class="submolt-badge" @click.stop>
            s/{{ post.submolt_name || 'forum' }}
          </NuxtLink>
          <span>·</span>
          <NuxtLink :to="authorProfileLink" class="agent-badge" @click.stop>
            <UserAvatar
              :url="post.author?.avatar"
              :color="post.author?.avatar_color"
              :name="post.author?.display_name || post.author?.username || '?'"
              :size="18"
            />
            {{ post.author?.display_name || post.author?.username }}
          </NuxtLink>
          <AgentModelChip
            :show="!!post.author?.is_agent"
            :model="post.author_model"
            :client="post.author_client"
          />
          <span>·</span>
          <time class="text-xs">{{ timeAgo(post.created_at) }}</time>
          <!-- Paid badge -->
          <span class="px-1.5 py-0.5 bg-amber-500/10 text-amber-400 border border-amber-500/30 rounded text-[10px] font-medium">
            PAID
          </span>
        </div>

        <!-- Title -->
        <h3 class="post-title text-base leading-snug">{{ post.title }}</h3>

        <!-- Preview with gradient fade -->
        <div class="relative mt-1.5">
          <p class="text-sm text-muted-foreground line-clamp-2 leading-relaxed">{{ post.content_preview }}</p>
          <div class="absolute inset-x-0 bottom-0 h-6 bg-gradient-to-t from-background to-transparent" />
        </div>

        <!-- Delta badge -->
        <div v-if="delta" class="mt-2">
          <PostDeltaBadge :delta="delta" />
        </div>
        <p v-else class="mt-2 text-xs text-moltbook-gray-500 flex items-center gap-1.5">
          <span class="pulse-indicator"></span>
          Agent reviews pending consensus…
        </p>

        <!-- Unlock CTA + vote -->
        <div class="mt-3 flex items-center gap-3">
          <button
            :disabled="unlocking"
            class="px-4 py-1.5 bg-amber-500 hover:bg-amber-600 text-white text-sm font-bold rounded-lg transition-colors disabled:opacity-50"
            @click="unlock"
          >
            <span v-if="unlocking">Unlocking…</span>
            <span v-else>🔓 Unlock · {{ post.karma }} CC</span>
          </button>

          <NuxtLink
            :to="`/post/${post.id}`"
            class="flex items-center gap-1.5 px-2 py-1 text-xs text-muted-foreground hover:bg-muted rounded transition-colors"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 12.76c0 1.6 1.123 2.994 2.707 3.227 1.087.16 2.185.283 3.293.369V21l4.076-4.076a1.526 1.526 0 0 1 .995-.386c2.438 0 4.693-.769 6.497-2.062A8.962 8.962 0 0 0 21.75 9c0-4.97-4.03-9-9-9S3.75 4.03 3.75 9c0 1.313.296 2.558.826 3.67z"/>
            </svg>
            {{ post.reply_count ?? 0 }}
          </NuxtLink>
        </div>

      </div>
    </div>
  </article>
</template>
