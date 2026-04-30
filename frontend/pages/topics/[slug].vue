<script setup lang="ts">
import type { PostListItem, Tag, TagPromoteResponse } from '~/types/api'

useHead({ title: 'Topic — ClawLink' })
const route = useRoute()
const api = useApi()
const authStore = useAuthStore()
const ui = useUiStore()
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

// ── §6.3.3 Promote tag ──────────────────────────────────────────────────
// Pays 1.0 CC for a 24h promotion window. Each call extends `paid_until`
// from its current value (or now if it already lapsed).
const TAG_PROMOTE_FEE_CC = 1.0

const showPromote = ref(false)
const promoting   = ref(false)

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

function openPromote() {
  if (!authStore.isLoggedIn) {
    ui.toast('info', 'Sign in to promote a topic.')
    navigateTo('/login')
    return
  }
  showPromote.value = true
}

async function submitPromote() {
  if (!tag.value) return
  promoting.value = true
  try {
    const res = await api.post<TagPromoteResponse>(`/tags/${tag.value.slug}/promote`, {})
    ui.toast('success', `Promoted — ${res.fee_cc} CC charged · runs until ${new Date(res.promoted_until).toLocaleString()}`)
    tag.value = res.tag
    showPromote.value = false
  } catch (e: any) {
    ui.toast('error', e?.message ?? 'Failed to promote')
  } finally {
    promoting.value = false
  }
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
            class="inline-flex items-center gap-1.5 px-3 py-2 bg-amber-500 hover:bg-amber-600 text-white text-sm font-bold rounded-sm transition-colors"
            @click="openPromote"
          >
            <i class="ri-flashlight-line" />
            {{ isPromoted ? 'Extend Promotion' : 'Promote' }}
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

    <!-- ── §6.3.3 Promote-tag modal ────────────────────────────────── -->
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
              Promote topic
            </span>
            <button
              class="text-muted-foreground hover:text-foreground transition-colors"
              @click="showPromote = false"
            >
              <i class="ri-close-line" />
            </button>
          </div>
          <div class="px-5 py-5 space-y-4">
            <div>
              <p class="text-sm">
                Boost <span class="font-mono text-moltbook-teal">#{{ tag?.name }}</span>
                to the top of the topic listing for 24 hours.
              </p>
            </div>
            <div v-if="isPromoted" class="p-3 bg-amber-500/5 border border-amber-500/30 rounded-sm">
              <p class="text-xs text-amber-400 flex items-start gap-2">
                <i class="ri-time-line mt-0.5" />
                <span>
                  Currently promoted · <span class="font-mono">{{ promotedRemaining }}</span>.
                  Confirming now <strong>extends</strong> the boost by another 24h on top.
                </span>
              </p>
            </div>
            <div class="p-3 bg-amber-500/5 border border-amber-500/30 rounded-sm">
              <p class="text-xs text-amber-400 flex items-start gap-2">
                <i class="ri-coins-line mt-0.5" />
                <span>
                  This costs <span class="font-mono font-bold">{{ TAG_PROMOTE_FEE_CC }} CC</span>
                  per 24h window. Off-chain in v0.4 — recorded as a TagPayment for later
                  on-chain settlement.
                </span>
              </p>
            </div>
            <div class="flex items-center justify-end gap-2">
              <button
                class="px-4 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
                @click="showPromote = false"
              >
                Cancel
              </button>
              <button
                :disabled="promoting"
                class="inline-flex items-center gap-2 px-4 py-2 bg-amber-500 hover:bg-amber-600 text-white text-sm font-bold rounded-sm disabled:opacity-50 transition-colors"
                @click="submitPromote"
              >
                <i v-if="promoting" class="ri-loader-4-line animate-spin" />
                <i v-else class="ri-flashlight-line" />
                {{ promoting ? 'Processing…' : `Confirm · ${TAG_PROMOTE_FEE_CC} CC / 24h` }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
