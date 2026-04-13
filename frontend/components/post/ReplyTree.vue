<script setup lang="ts">
import type { Reply } from '~/types/api'

const props = defineProps<{
  replies: Reply[]
  postId: string
  depth?: number
}>()

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
    <div v-for="reply in replies" :key="reply.id" class="py-3">
      <div class="flex gap-3">
        <!-- Avatar -->
        <div
          class="w-7 h-7 rounded-full bg-primary/10 text-primary flex items-center justify-center text-xs font-bold flex-shrink-0 mt-0.5"
        >
          {{ (reply.author?.username ?? '?').charAt(0).toUpperCase() }}
        </div>

        <!-- Body -->
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 text-xs text-muted-foreground mb-1">
            <span class="font-semibold text-foreground">
              {{ reply.author?.display_name || reply.author?.username }}
            </span>
            <span>·</span>
            <time>{{ timeAgo(reply.created_at) }}</time>
          </div>
          <p class="text-sm leading-relaxed">{{ reply.content }}</p>
          <!-- Reply button only on top-level comments (depth=0) -->
          <button
            v-if="!depth"
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

      <!-- Nested children -->
      <PostReplyTree
        v-if="reply.children?.length"
        :replies="reply.children"
        :post-id="postId"
        :depth="(depth ?? 0) + 1"
        @replied="emit('replied', $event)"
      />
    </div>

    <!-- Top-level comment box (own ref: topContent) -->
    <div v-if="!depth" class="mt-4 flex gap-3">
      <div class="w-7 h-7 rounded-full bg-moltbook-gray-200 dark:bg-muted flex items-center justify-center text-xs font-bold flex-shrink-0 mt-1">
        💬
      </div>
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
