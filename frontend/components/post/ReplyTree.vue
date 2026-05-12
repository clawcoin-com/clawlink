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

const visibleReplies = computed<Reply[]>(() => {
  return props.replies
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

const childCursors = ref<Record<string, string>>({})
const loadingChildren = ref(new Set<string>())

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

function visibleChildCount(reply: Reply) {
  return reply.children?.length ?? 0
}

function hasMoreChildren(reply: Reply) {
  return visibleChildCount(reply) < (reply.child_count ?? 0)
}

function childCursor(reply: Reply) {
  const existing = childCursors.value[reply.id]
  if (existing) return existing
  const children = reply.children ?? []
  return children.length ? children[children.length - 1].created_at : undefined
}

async function loadChildren(reply: Reply) {
  if (loadingChildren.value.has(reply.id)) return
  loadingChildren.value.add(reply.id)
  try {
    const res = await api.getList<Reply>(`/posts/${props.postId}/replies`, {
      parent_id: reply.id,
      limit: 10,
      cursor: childCursor(reply),
    })
    reply.children = [...(reply.children ?? []), ...(res.data ?? [])]
    childCursors.value[reply.id] = res.cursor ?? ''
  } finally {
    loadingChildren.value.delete(reply.id)
  }
}

async function submitInlineReply(parentId: string) {
  if (!authStore.isLoggedIn) { ui.toast('info', 'Login required'); return }
  if (!inlineContent.value.trim()) return
  inlineSubmitting.value = true
  try {
    const reply = await api.post<Reply>(`/posts/${props.postId}/replies`, {
      content: inlineContent.value,
      parent_id: parentId,
    })
    reply.child_count = reply.child_count ?? 0
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
    reply.child_count = reply.child_count ?? 0
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

      <div v-if="hasMoreChildren(reply)" class="ml-10 mt-2">
        <button
          class="text-xs text-moltbook-teal hover:underline disabled:opacity-50"
          :disabled="loadingChildren.has(reply.id)"
          @click="loadChildren(reply)"
        >
          ↳ {{ visibleChildCount(reply) === 0 ? 'Show' : 'Load more' }}
          {{ (reply.child_count ?? 0) - visibleChildCount(reply) }} replies
        </button>
      </div>

      <!-- Collapsed deep thread teaser -->
      <button
        v-if="isCollapsed(reply, depth ?? 0)"
        class="ml-10 mt-2 text-xs text-moltbook-teal hover:underline"
        @click="expand(reply.id)"
      >
        ↳ Show {{ descendantCount(reply) }} loaded nested replies
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
