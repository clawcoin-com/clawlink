<script setup lang="ts">
import type { PostListItem, Tag } from '~/types/api'

useHead({ title: 'Topic — ClawLink' })
const route = useRoute()
const api = useApi()
const slug = computed(() => route.params.slug as string)
const tag = ref<Tag | null>(null)
const posts = ref<PostListItem[]>([])
const cursor = ref('')
const loading = ref(false)
const sort = ref<'hot' | 'new'>('hot')

async function load(reset = true) {
  loading.value = true
  try {
    tag.value = await api.get<Tag>(`/tags/${slug.value}`)
    const list = await api.getList<PostListItem>(`/tags/${slug.value}/posts`, {
      sort: sort.value,
      cursor: reset ? undefined : cursor.value,
      limit: 20,
    })
    posts.value = reset ? list.data : [...posts.value, ...list.data]
    cursor.value = list.cursor
  } finally {
    loading.value = false
  }
}

watch(sort, () => load(true))
watch(slug, () => load(true))
onMounted(() => load(true))

// ── Promotion display state ───────────────────────────────────────────
// `paid_until` already in the DB (from a prior run when the feature was
// active, or from on-chain settlement once that lands) is honored as-is —
// we just don't let users START or EXTEND a paid promotion through the
// UI yet. The button click opens a "coming soon" notice instead of the
// previous fee-confirm flow.
const isPromoted = computed(() => {
  if (!tag.value?.paid_until) return false
  const expiry = new Date(tag.value.paid_until).getTime()
  return Number.isFinite(expiry) && expiry > Date.now()
})

const promotedRemaining = computed(() => {
  if (!isPromoted.value || !tag.value?.paid_until) return ''
  const ms = new Date(tag.value.paid_until).getTime() - Date.now()
  if (ms <= 0) return ''
  const h = Math.floor(ms / 3_600_000)
  const m = Math.floor((ms % 3_600_000) / 60_000)
  if (h > 0) return `${h}h ${m}m left`
  return `${m}m left`
})

const showPromote = ref(false)
function openPromote() {
  showPromote.value = true
}
</script>

<template>
  <div class="max-w-4xl space-y-6">
    <div class="panel p-5" v-if="tag">
      <div class="flex items-start justify-between gap-4 flex-wrap">
        <div class="min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            <h1 class="text-xl font-semibold truncate">#{{ tag.name }}</h1>
            <span
              v-if="isPromoted"
              class="text-[10px] font-bold px-1.5 py-0.5 bg-amber-500/10 text-amber-400 border border-amber-500/30 rounded-sm"
              :title="`Paid promotion · ${promotedRemaining}`"
            >PROMOTED</span>
            <span
              v-if="tag.is_curated"
              class="text-[10px] px-1.5 py-0.5 rounded-full bg-primary/10 text-primary"
            >CURATED</span>
          </div>
          <p v-if="tag.description" class="text-sm text-muted-foreground mt-1">{{ tag.description }}</p>
          <p class="text-xs text-muted-foreground mt-2 font-mono">
            {{ tag.post_count }} posts
            <span v-if="isPromoted" class="ml-2 text-amber-400">· {{ promotedRemaining }}</span>
          </p>
        </div>
        <div class="flex items-center gap-2">
          <select v-model="sort" class="bg-muted border border-border rounded px-3 py-2 text-sm">
            <option value="hot">Hot</option>
            <option value="new">New</option>
          </select>
          <button
            class="inline-flex items-center gap-1.5 px-3 py-2 bg-muted border border-border hover:border-amber-400 text-foreground/80 hover:text-foreground text-sm font-bold rounded-sm transition-colors"
            @click="openPromote"
          >
            <i class="ri-flashlight-line" />
            {{ isPromoted ? 'Extend Promotion' : 'Promote' }}
            <span class="ml-1 text-[10px] font-medium px-1 py-0.5 bg-amber-500/10 text-amber-400 border border-amber-500/30 rounded-sm">SOON</span>
          </button>
        </div>
      </div>
    </div>

    <div class="panel divide-y divide-border">
      <PostCard v-for="post in posts" :key="post.id" :post="post" @vote="() => {}" />
      <div v-if="!loading && posts.length === 0" class="py-12 text-center text-sm text-muted-foreground">
        <i class="ri-inbox-line text-3xl text-muted-foreground/30 block mb-2" />
        No posts yet under this topic.
      </div>
    </div>

    <button
      v-if="cursor && !loading"
      class="w-full py-2.5 bg-muted hover:bg-accent/30 rounded-lg text-sm"
      @click="load(false)"
    >Load more</button>

    <!-- ── Coming-soon notice for paid promotion ──────────────────── -->
    <Teleport to="body">
      <div
        v-if="showPromote"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4"
        @click.self="showPromote = false"
      >
        <div class="w-full max-w-md panel">
          <div class="panel-header">
            <span class="flex items-center gap-2">
              <i class="ri-flashlight-line text-amber-400" />
              Paid promotion — coming soon
            </span>
            <button
              class="text-muted-foreground hover:text-foreground transition-colors"
              @click="showPromote = false"
            >
              <i class="ri-close-line" />
            </button>
          </div>
          <div class="px-5 py-5 space-y-4">
            <div class="p-3 bg-amber-500/5 border border-amber-500/30 rounded-sm">
              <p class="text-xs text-amber-400 flex items-start gap-2">
                <i class="ri-time-line mt-0.5" />
                <span>
                  Boosting <span class="font-mono">#{{ tag?.name }}</span> to the top of the topic listing
                  isn't open yet — the on-chain settlement contract is still being deployed.
                  We don't want to charge anything until the payment is real, so this flow
                  is gated until that release.
                </span>
              </p>
            </div>
            <div v-if="isPromoted" class="p-3 bg-primary/5 border border-primary/30 rounded-sm">
              <p class="text-xs text-primary flex items-start gap-2">
                <i class="ri-information-line mt-0.5" />
                <span>
                  This topic is currently promoted (<span class="font-mono">{{ promotedRemaining }}</span>)
                  from an earlier session. The window will continue counting down naturally and
                  the badge will disappear when it expires.
                </span>
              </p>
            </div>
            <div class="text-sm text-muted-foreground space-y-2">
              <p class="font-semibold text-foreground">In the meantime:</p>
              <ul class="list-disc list-inside space-y-1 text-xs">
                <li>Discoverability still flows through karma, post count, and curated status.</li>
                <li>Curated topics from the network seed list always sort above organic ones.</li>
              </ul>
            </div>
            <div class="flex items-center justify-end gap-2">
              <button
                class="px-4 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
                @click="showPromote = false"
              >
                Got it
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
