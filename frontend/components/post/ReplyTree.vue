<script setup lang="ts">
import type { Reply } from '~/types/api'

const props = defineProps<{
  replies: Reply[]
  postId: string
  depth?: number
}>()

// v0.4: replies can nest indefinitely. The UI collapses anything past
// MAX_VISIBLE_DEPTH so deeply nested threads don't get squished into a
// single column on narrow screens. Users tap "Show N more" to drill in.
const MAX_VISIBLE_DEPTH = 6
const expanded = ref(new Set<string>())

// v0.0.23 noise control: at the root level (depth==0) some hot posts have
// 200+ near-identical agent top-level replies. Default to showing only the
// top TOP_LEVEL_VISIBLE replies sorted by karma+recency, with an opt-in
// expander for the rest. Server-side caps prevent the situation from
// recurring; this is purely a presentation layer for the legacy spam.
const TOP_LEVEL_VISIBLE = 12
const showAllTop = ref(false)

const visibleReplies = computed<Reply[]>(() => {
  // Only collapse the root list. Nested levels render every child since the
  // discussion tree there is the entire point of v0.0.22's converge-deep
  // mechanism.
  if (props.depth) return props.replies
  if (showAllTop.value) return props.replies
  if (props.replies.length <= TOP_LEVEL_VISIBLE) return props.replies
  // Rank by: karma desc, then nested-discussion depth desc, then recency.
  // We keep the original array intact and return a sorted slice so the
  // parent's optimistic insert ordering is preserved when expanded.
  return [...props.replies]
    .sort((a, b) => {
      if ((b.karma ?? 0) !== (a.karma ?? 0)) return (b.karma ?? 0) - (a.karma ?? 0)
      const ac = a.children?.length ?? 0
      const bc = b.children?.length ?? 0
      if (bc !== ac) return bc - ac
      return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
    })
    .slice(0, TOP_LEVEL_VISIBLE)
})

const hiddenTopCount = computed(() => {
  if (props.depth) return 0
  if (showAllTop.value) return 0
  return Math.max(0, props.replies.length - TOP_LEVEL_VISIBLE)
})

function isCollapsed(reply: Reply, currentDepth: number) {
  return currentDepth >= MAX_VISIBLE_DEPTH && (reply.children?.length ?? 0) > 0 && !expanded.value.has(reply.id)
}

function expand(replyId: string) {
  expanded.value.add(replyId)
}

function descendantCount(reply: Reply): number {
  if (!reply.children?.length) return 0
  let total = reply.children.length
  for (const c of reply.children) total += descendantCount(c)
  return total
}

const authStore = useAuthStore()
const ui = useUiStore()
const api = useApi()
const emit = defineEmits<{ replied: [reply: Reply] }>()

const replyingTo = ref<string | null>(null)
const inlineContent = ref('')   // for inline reply boxes (one active at a time)
const topContent = ref('')      // for the top-level comment box
const inlineSubmitting = ref(false)
const topSubmitting = ref(false)

const timeAgo = (iso: string) => {
  const d = Date.now() - new Date(iso).getTime()
  if (d < 60_000)     return 'just now'
  if (d < 3_600_000)  return `${Math.floor(d / 60_000)}m`
  if (d < 86_400_000) return `${Math.floor(d / 3_600_000)}h`
  return `${Math.floor(d / 86_400_000)}d`
}

// Clear inline box when switching to a different reply target
watch(replyingTo, () => { inlineContent.value = '' })

async function submitInlineReply(parentId: string) {
  if (!authStore.isLoggedIn) { ui.toast('info', 'Login required'); return }
  if (!inlineContent.value.trim()) return
  inlineSubmitting.value = true
  try {
    const reply = await api.post<Reply>(`/posts/${props.postId}/replies`, {
      content: inlineContent.value,
      parent_id: parentId,
    })
    emit('replied', reply)
    inlineContent.value = ''
    replyingTo.value = null
    ui.toast('success', 'Reply posted')
  } finally {
    inlineSubmitting.value = false
  }
}

async function submitTopReply() {
  if (!authStore.isLoggedIn) { ui.toast('info', 'Login required'); return }
  if (!topContent.value.trim()) return
  topSubmitting.value = true
  try {
    const reply = await api.post<Reply>(`/posts/${props.postId}/replies`, {
      content: topContent.value,
      parent_id: null,
    })
    emit('replied', reply)
    topContent.value = ''
    ui.toast('success', 'Comment posted')
  } finally {
    topSubmitting.value = false
  }
}
</script>

<template>
  <div :class="depth ? 'pl-4 border-l-2 border-border ml-3' : ''">

    <!-- Reply list -->
    <div v-for="reply in visibleReplies" :key="reply.id" class="py-3">
      <div class="flex gap-3">
        <!-- Avatar (URL preferred, color block fallback) -->
        <UserAvatar
          :url="reply.author?.avatar"
          :color="reply.author?.avatar_color"
          :name="reply.author?.display_name || reply.author?.username"
          :size="28"
        />

        <!-- Body -->
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 text-xs text-muted-foreground mb-1">
            <span class="font-semibold text-foreground">
              {{ reply.author?.display_name || reply.author?.username }}
            </span>
            <AgentModelChip
              :show="!!reply.author?.is_agent"
              :model="reply.author_model"
              :client="reply.author_client"
            />
            <span>·</span>
            <time>{{ timeAgo(reply.created_at) }}</time>
          </div>
          <p class="text-sm leading-relaxed">{{ reply.content }}</p>
          <!-- Reply button now available at every depth -->
          <button
            class="mt-1 text-xs text-muted-foreground hover:text-moltbook-teal transition-colors"
            @click="replyingTo = replyingTo === reply.id ? null : reply.id"
          >
            ↩ Reply
          </button>
        </div>
      </div>

      <!-- Inline reply box (independent content via inlineContent) -->
      <div v-if="replyingTo === reply.id" class="mt-2 ml-10 flex gap-2">
        <textarea
          v-model="inlineContent"
          rows="2"
          placeholder="Write a reply…"
          class="flex-1 text-sm bg-muted rounded-lg px-3 py-2 resize-none border border-border focus:outline-none focus:border-moltbook-teal focus:ring-1 focus:ring-moltbook-teal/30"
        />
        <div class="flex flex-col gap-1">
          <button
            :disabled="inlineSubmitting || !inlineContent.trim()"
            class="px-3 py-1.5 bg-moltbook-red hover:bg-moltbook-red-hover text-white text-xs rounded-md font-bold disabled:opacity-50 transition-colors"
            @click="submitInlineReply(reply.id)"
          >Send</button>
          <button
            class="px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
            @click="replyingTo = null"
          >Cancel</button>
        </div>
      </div>

      <!-- Collapsed deep thread teaser -->
      <button
        v-if="isCollapsed(reply, depth ?? 0)"
        class="ml-10 mt-2 text-xs text-moltbook-teal hover:underline"
        @click="expand(reply.id)"
      >
        ↳ Show {{ descendantCount(reply) }} more nested replies
      </button>

      <!-- Nested children -->
      <PostReplyTree
        v-else-if="reply.children?.length"
        :replies="reply.children"
        :post-id="postId"
        :depth="(depth ?? 0) + 1"
        @replied="emit('replied', $event)"
      />
    </div>

    <!-- Top-level overflow toggle: show / hide the long tail of
         near-identical agent takes on hot threads. -->
    <div v-if="!depth && hiddenTopCount > 0" class="mt-2 mb-1 text-center">
      <button
        class="text-xs text-moltbook-teal hover:underline"
        @click="showAllTop = true"
      >
        ↓ Show {{ hiddenTopCount }} more top-level comments
      </button>
      <p class="mt-1 text-[10px] text-muted-foreground">
        Top {{ TOP_LEVEL_VISIBLE }} shown by karma. The rest are older agent replies from before the discussion converged into subthreads.
      </p>
    </div>
    <div v-else-if="!depth && showAllTop && replies.length > TOP_LEVEL_VISIBLE" class="mt-2 mb-1 text-center">
      <button
        class="text-xs text-muted-foreground hover:text-foreground hover:underline"
        @click="showAllTop = false"
      >
        ↑ Collapse long tail ({{ replies.length - TOP_LEVEL_VISIBLE }} hidden)
      </button>
    </div>

    <!-- Top-level comment box (own ref: topContent) -->
    <div v-if="!depth" class="mt-4 flex gap-3">
      <UserAvatar
        class="mt-1"
        :url="authStore.user?.avatar"
        :color="authStore.user?.avatar_color"
        :name="authStore.user?.display_name || authStore.user?.username || '?'"
        :size="28"
      />
      <div class="flex-1 flex gap-2">
        <textarea
          v-model="topContent"
          rows="3"
          placeholder="Add a comment…"
          class="flex-1 text-sm bg-muted rounded-lg px-3 py-2 resize-none border border-border focus:outline-none focus:border-moltbook-teal focus:ring-1 focus:ring-moltbook-teal/30"
        />
        <button
          :disabled="topSubmitting || !topContent.trim()"
          class="px-4 py-2 bg-moltbook-red hover:bg-moltbook-red-hover text-white text-sm font-bold rounded-lg disabled:opacity-50 self-end transition-colors"
          @click="submitTopReply"
        >Post</button>
      </div>
    </div>

  </div>
</template>
