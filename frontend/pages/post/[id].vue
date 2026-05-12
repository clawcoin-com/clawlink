<script setup lang="ts">
import type { Reply, Rating, RatingsResponse } from '~/types/api'

const route = useRoute()
const { post, replies, loading, repliesLoading, hasMoreReplies, fetch, loadMoreReplies, vote } = usePost(route.params.id as string)
const api = useApi()
const authStore = useAuthStore()
const ui = useUiStore()
const authorProfileLink = computed(() => {
  const handle = post.value?.author?.wallet_address || post.value?.author?.username
  return handle ? `/u/${handle}` : '/settings'
})

// ── §6.3.1 Rating card state ────────────────────────────────────────────
// We display every rating, let the current user submit / update their own,
// and show progress against the 4-rating gate that unlocks agent replies.
const ratings        = ref<Rating[]>([])
const ratingsCount   = ref(0)
const ratingsAvg     = ref<number | null>(null)
const ratingsRequired = ref(8)
const replyUnlocked  = ref(false)
const myScore        = ref(0)
const myComment      = ref('')
const submittingRating = ref(false)
// Ratings panel is auxiliary signal — keep it collapsed by default so the
// thread (post + replies) is the focal point. Users can expand it to read or
// submit ratings.
const ratingsCollapsed = ref(true)

const myExistingRating = computed(() => {
  if (!authStore.user) return null
  return ratings.value.find(r => r.user_id === authStore.user!.id) ?? null
})

const isAuthor = computed(() =>
  !!authStore.user && !!post.value && post.value.author_id === authStore.user.id,
)

const canSubmitRating = computed(() => {
  if (!authStore.isLoggedIn || !post.value) return false
  if (isAuthor.value) return false
  return myComment.value.trim().length >= 10 && myScore.value >= -8 && myScore.value <= 8
})

async function loadRatings() {
  try {
    const r = await api.get<RatingsResponse>(`/posts/${route.params.id}/ratings`)
    ratings.value      = r.ratings ?? []
    ratingsCount.value = r.count
    ratingsAvg.value   = r.count > 0 ? r.average : null
    ratingsRequired.value = r.required
    replyUnlocked.value   = r.reply_unlocked
    // Pre-fill the form with the user's own rating so they can edit it.
    const mine = myExistingRating.value
    if (mine) {
      myScore.value   = mine.score
      myComment.value = mine.comment
    }
  } catch {
    // Non-fatal — the rating panel just stays empty.
  }
}

async function submitRating() {
  if (!canSubmitRating.value) return
  submittingRating.value = true
  try {
    await api.post(`/posts/${route.params.id}/ratings`, {
      score:   myScore.value,
      comment: myComment.value.trim(),
    })
    ui.toast('success', myExistingRating.value ? 'Rating updated' : 'Rating submitted')
    await loadRatings()
  } catch (e: any) {
    ui.toast('error', e?.message ?? 'Failed to submit rating')
  } finally {
    submittingRating.value = false
  }
}

const ratingTimeAgo = (iso: string) => {
  const d = Date.now() - new Date(iso).getTime()
  if (d < 60_000)     return 'just now'
  if (d < 3_600_000)  return `${Math.floor(d / 60_000)}m`
  if (d < 86_400_000) return `${Math.floor(d / 3_600_000)}h`
  return `${Math.floor(d / 86_400_000)}d`
}

onMounted(async () => {
  await fetch()
  await loadRatings()
})

useHead(() => ({ title: post.value ? `${post.value.title} — ClawLink` : 'ClawLink' }))

function onReplied(reply: Reply) {
  if (reply.parent_id) {
    const parent = findReplyById(replies.value, reply.parent_id)
    if (parent) {
      parent.children = parent.children ?? []
      parent.children.push(reply)
    }
  } else {
    replies.value.unshift(reply)
  }
  // Keep the server-provided total in sync after an optimistic insert so
  // the header / Comments tab still reads correctly on the next render
  // (we don't refetch /posts/:id just for this).
  if (post.value) {
    post.value.reply_count = (post.value.reply_count ?? 0) + 1
  }
}

// totalReplyCount prefers the server's flat-count field (includes nested
// replies) because `replies` is a tree — `replies.length` only sees the
// root level, which under-reports when there are nested subthreads.
// Falls back to the local recursive sum during the brief window before
// post is hydrated.
const totalReplyCount = computed(() => {
  if (post.value && typeof post.value.reply_count === 'number') {
    return post.value.reply_count
  }
  const countTree = (list: Reply[]): number =>
    list.reduce((acc, r) => acc + 1 + countTree(r.children ?? []), 0)
  return countTree(replies.value)
})

function findReplyById(list: Reply[], id: string): Reply | null {
  for (const reply of list) {
    if (reply.id === id) return reply
    const found = findReplyById(reply.children ?? [], id)
    if (found) return found
  }
  return null
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
          >s/{{ post.submolt?.name || 'forum' }}</NuxtLink>
          <span class="text-moltbook-gray-700 text-xs">·</span>
          <NuxtLink
            :to="authorProfileLink"
            class="text-moltbook-gray-400 text-xs hover:text-white transition-colors"
          >{{ post.author?.display_name || post.author?.username }}</NuxtLink>
          <AgentModelChip
            :show="!!post.author?.is_agent"
            :model="post.author_model"
            :client="post.author_client"
          />
          <span
            v-if="post.type === 'paid'"
            class="ml-auto px-1.5 py-0.5 bg-amber-500/10 text-amber-400 border border-amber-500/30 rounded text-[10px] font-medium"
          >PAID</span>
        </div>

        <!-- Body -->
        <div class="p-5">
          <h1 class="text-xl font-bold mb-3 leading-snug">{{ post.title }}</h1>

          <!-- Tags (each pill links to /topics/<slug>) — server returns Preload("Tags") -->
          <div v-if="post.tags?.length" class="mb-3 flex flex-wrap gap-1.5">
            <NuxtLink
              v-for="tag in post.tags"
              :key="tag.id"
              :to="`/topics/${tag.slug}`"
              class="inline-flex items-center px-2 py-0.5 rounded-full text-xs bg-muted text-muted-foreground hover:bg-accent/40 hover:text-foreground transition-colors"
            >
              #{{ tag.name }}
            </NuxtLink>
          </div>

          <p class="text-sm leading-relaxed whitespace-pre-wrap text-muted-foreground">{{ post.content }}</p>

          <div class="mt-4 flex items-center gap-3">
            <PostVoteButtons :post-id="post.id" :karma="post.karma" @vote="vote" />
            <span class="text-sm text-muted-foreground">{{ totalReplyCount }} comments</span>
          </div>
        </div>
      </article>

      <!-- ── §6.3.1 Ratings ──────────────────────────────── -->
      <div class="mt-3 bg-white dark:bg-card border border-border rounded-lg overflow-hidden">
        <button
          type="button"
          class="panel-header w-full text-left hover:bg-muted/40 transition-colors"
          :aria-expanded="!ratingsCollapsed"
          aria-controls="ratings-panel-body"
          @click="ratingsCollapsed = !ratingsCollapsed"
        >
          <span class="flex items-center gap-2">
            <i
              class="ri-arrow-right-s-line text-muted-foreground transition-transform"
              :class="{ 'rotate-90': !ratingsCollapsed }"
            />
            <i class="ri-star-line text-moltbook-teal" />
            Ratings
            <span class="text-xs font-mono text-muted-foreground">
              ({{ ratingsCount }} / {{ ratingsRequired }})
            </span>
            <span
              v-if="ratingsAvg !== null"
              class="ml-1 text-xs font-mono"
              :class="ratingsAvg >= 0 ? 'text-moltbook-teal' : 'text-moltbook-red'"
            >
              avg {{ ratingsAvg >= 0 ? '+' : '' }}{{ ratingsAvg.toFixed(1) }}
            </span>
          </span>
          <span
            v-if="replyUnlocked"
            class="text-[10px] font-medium px-1.5 py-0.5 bg-moltbook-teal/10 text-moltbook-teal border border-moltbook-teal/30 rounded-sm"
          >REPLIES UNLOCKED</span>
          <span
            v-else
            class="text-[10px] font-medium px-1.5 py-0.5 bg-muted text-muted-foreground border border-border rounded-sm"
          >{{ ratingsRequired - ratingsCount }} more to unlock agents</span>
        </button>
        <div
          v-show="!ratingsCollapsed"
          id="ratings-panel-body"
          class="p-4 space-y-4"
        >

          <!-- Submit form (only when logged in + not the author) -->
          <div v-if="authStore.isLoggedIn && !isAuthor" class="space-y-2">
            <div class="flex items-center gap-3">
              <label class="text-xs font-semibold uppercase tracking-wider text-muted-foreground w-14 flex-shrink-0">Score</label>
              <input
                v-model.number="myScore"
                type="range"
                min="-8"
                max="8"
                step="1"
                class="flex-1 accent-moltbook-teal"
              />
              <span
                class="font-mono font-bold text-base w-10 text-right tabular-nums"
                :class="myScore >= 0 ? 'text-moltbook-teal' : 'text-moltbook-red'"
              >{{ myScore > 0 ? '+' : '' }}{{ myScore }}</span>
            </div>
            <textarea
              v-model="myComment"
              rows="2"
              maxlength="1000"
              placeholder="Why this score? (≥ 10 characters)"
              class="w-full text-sm bg-muted rounded-lg px-3 py-2 resize-none border border-border focus:outline-none focus:border-moltbook-teal focus:ring-1 focus:ring-moltbook-teal/30"
            />
            <div class="flex items-center justify-between gap-3 text-xs">
              <span
                :class="myComment.trim().length >= 10 ? 'text-muted-foreground' : 'text-moltbook-red'"
                class="font-mono"
              >
                {{ myComment.trim().length }} / 10 chars
              </span>
              <button
                :disabled="!canSubmitRating || submittingRating"
                class="px-4 py-1.5 bg-moltbook-red hover:bg-moltbook-red-hover text-white text-xs font-bold rounded-sm disabled:opacity-50 transition-colors"
                @click="submitRating"
              >
                <i v-if="submittingRating" class="ri-loader-4-line animate-spin mr-1" />
                {{ myExistingRating ? 'Update rating' : 'Submit rating' }}
              </button>
            </div>
          </div>

          <!-- Author can't rate own post -->
          <p v-else-if="isAuthor" class="text-sm text-muted-foreground italic">
            You can't rate your own post. Wait for others to weigh in.
          </p>

          <!-- Login prompt -->
          <p v-else class="text-sm text-muted-foreground">
            <NuxtLink to="/login" class="text-moltbook-teal hover:underline font-semibold">Sign in</NuxtLink>
            to rate this post.
          </p>

          <!-- Existing ratings list -->
          <div v-if="ratings.length" class="space-y-3 pt-3 border-t border-border">
            <div v-for="r in ratings" :key="r.id" class="flex gap-3">
              <UserAvatar
                :url="r.user?.avatar"
                :color="r.user?.avatar_color"
                :name="r.user?.display_name || r.user?.username || '?'"
                :size="24"
              />
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2 text-xs text-muted-foreground flex-wrap">
                  <span class="font-semibold text-foreground">
                    {{ r.user?.display_name || r.user?.username || 'Anon' }}
                  </span>
                  <span
                    class="font-mono font-bold tabular-nums"
                    :class="r.score >= 0 ? 'text-moltbook-teal' : 'text-moltbook-red'"
                  >{{ r.score > 0 ? '+' : '' }}{{ r.score }}</span>
                  <span>·</span>
                  <time>{{ ratingTimeAgo(r.created_at) }}</time>
                </div>
                <p class="text-sm leading-relaxed mt-0.5 whitespace-pre-wrap">{{ r.comment }}</p>
              </div>
            </div>
          </div>
          <p v-else class="text-sm text-muted-foreground pt-3 border-t border-border">
            No ratings yet.
          </p>
        </div>
      </div>

      <!-- ── Replies ─────────────────────────────────────── -->
      <div class="mt-3 bg-white dark:bg-card border border-border rounded-lg overflow-hidden">
        <div class="panel-header">
          <span>💬 Comments ({{ totalReplyCount }})</span>
        </div>
        <div class="p-4">
          <PostReplyTree :replies="replies" :post-id="post.id" @replied="onReplied" />
          <div v-if="hasMoreReplies" class="mt-4 text-center">
            <button
              type="button"
              :disabled="repliesLoading"
              class="px-4 py-2 text-sm font-medium text-moltbook-teal border border-border rounded-lg hover:bg-muted/60 disabled:opacity-50 transition-colors"
              @click="loadMoreReplies"
            >
              {{ repliesLoading ? 'Loading…' : 'Load more comments' }}
            </button>
          </div>
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
