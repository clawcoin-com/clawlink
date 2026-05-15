<script setup lang="ts">
import type { Reply } from '~/types/api'

const props = defineProps<{
  replies: Reply[]
  postId: string
  depth?: number
}>()

// Keep indentation visible all the way down the tree so deep branches remain
// easy to read instead of flattening after a fixed depth.
const INDENT_CLASS = 'border-l border-border/80 ml-3 pl-4'
const COMPRESSED_CLASS = 'border-l border-dashed border-border/50'

// When a branch expands, opportunistically fetch one more generation so the
// user can see more of the subtree in a single interaction.
const PREFETCH_DESCENDANT_DEPTH = 1

const visibleReplies = computed<Reply[]>(() => {
  return props.replies
})

const collapsedReplies = ref<Record<string, boolean>>({})

function maxVisibleDepth(replies: Reply[], currentDepth: number): number {
  let maxDepth = currentDepth
  for (const reply of replies) {
    const replyDepth = currentDepth + 1
    maxDepth = Math.max(maxDepth, replyDepth)
    const childDepth = maxVisibleDepth(reply.children ?? [], replyDepth)
    maxDepth = Math.max(maxDepth, childDepth)
  }
  return maxDepth
}

const branchMaxDepth = computed(() => maxVisibleDepth(props.replies, props.depth ?? 0))

function treeClass() {
  if (!props.depth) return ''
  const depth = props.depth
  const tailStart = Math.max(4, branchMaxDepth.value - 1)
  if (depth <= 3 || depth >= tailStart) return INDENT_CLASS
  return COMPRESSED_CLASS
}

function isCompressedDepth(depth = props.depth ?? 0) {
  if (!depth) return false
  const tailStart = Math.max(4, branchMaxDepth.value - 1)
  return depth > 3 && depth < tailStart
}

function isCollapsed(reply: Reply) {
  return collapsedReplies.value[reply.id] ?? false
}

function toggleCollapsed(reply: Reply) {
  collapsedReplies.value = {
    ...collapsedReplies.value,
    [reply.id]: !isCollapsed(reply),
  }
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

function remainingChildCount(reply: Reply) {
  return Math.max(0, (reply.child_count ?? 0) - visibleChildCount(reply))
}

function childToggleLabel(reply: Reply) {
  const remaining = remainingChildCount(reply)
  if (visibleChildCount(reply) > 0) return 'Show more'
  return `Show ${remaining} ${remaining === 1 ? 'reply' : 'replies'}`
}

function childCursor(reply: Reply) {
  const existing = childCursors.value[reply.id]
  if (existing) return existing
  const children = reply.children ?? []
  return children.length ? children[children.length - 1].created_at : undefined
}

async function loadChildren(reply: Reply, prefetchDepth = PREFETCH_DESCENDANT_DEPTH) {
  if (loadingChildren.value.has(reply.id)) return
  loadingChildren.value.add(reply.id)
  try {
    const res = await api.getList<Reply>(`/posts/${props.postId}/replies`, {
      parent_id: reply.id,
      limit: 10,
      cursor: childCursor(reply),
    })
    const freshChildren = res.data ?? []
    reply.children = [...(reply.children ?? []), ...freshChildren]
    childCursors.value[reply.id] = res.cursor ?? ''

    if (prefetchDepth > 0 && freshChildren.length > 0) {
      await Promise.all(
        freshChildren
          .filter(child => (child.child_count ?? 0) > 0)
          .map(child => loadChildren(child, prefetchDepth - 1)),
      )
    }
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
  <div :class="treeClass()">
    <div v-if="isCompressedDepth()" class="mb-1 text-[10px] uppercase tracking-wide text-muted-foreground/70">
      · depth {{ props.depth }} compact thread view
    </div>

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

      <div v-if="reply.children?.length || hasMoreChildren(reply)" class="ml-10 mt-2 flex flex-wrap items-center gap-x-3 gap-y-1">
        <button
          v-if="reply.children?.length"
          class="text-xs text-muted-foreground hover:text-foreground transition-colors"
          @click="toggleCollapsed(reply)"
        >
          {{ isCollapsed(reply) ? '↳ Show replies' : '↳ Collapse replies' }}
        </button>
        <button
          v-if="hasMoreChildren(reply) && !isCollapsed(reply)"
          class="text-xs text-moltbook-teal hover:underline disabled:opacity-50"
          :disabled="loadingChildren.has(reply.id)"
          @click="loadChildren(reply)"
        >
          ↳ {{ childToggleLabel(reply) }}
        </button>
      </div>

      <!-- Nested children -->
      <PostReplyTree
        v-if="reply.children?.length && !isCollapsed(reply)"
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
