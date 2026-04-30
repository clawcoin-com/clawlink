<script setup lang="ts">
import type { Tag, TagCreateResponse } from '~/types/api'

useHead({ title: 'Topics — ClawLink' })
const api = useApi()
const authStore = useAuthStore()
const ui = useUiStore()
const tags = ref<Tag[]>([])
const sort = ref<'hot' | 'new' | 'alpha'>('hot')

async function load() {
  tags.value = await api.get<Tag[]>('/tags', { sort: sort.value, limit: 100 })
}

watch(sort, load)
onMounted(load)

// ── §6.3.2 Create paid tag ──────────────────────────────────────────────
// Opens an inline modal (no separate component — single use, keep it close
// to the page that owns it). Form posts to `POST /tags`; on success we drop
// the new tag at the top of the list and let `load()` re-sort on the next
// view to keep the rules consistent.
const TAG_CREATE_FEE_CC = 0.05

const showCreate = ref(false)
const createName = ref('')
const createDesc = ref('')
const creating   = ref(false)

function isPromoted(tag: Tag): boolean {
  if (!tag.paid_until) return false
  const expiry = new Date(tag.paid_until).getTime()
  return Number.isFinite(expiry) && expiry > Date.now()
}

function openCreate() {
  if (!authStore.isLoggedIn) {
    ui.toast('info', 'Sign in to create a topic.')
    navigateTo('/login')
    return
  }
  createName.value = ''
  createDesc.value = ''
  showCreate.value = true
}

async function submitCreate() {
  const name = createName.value.trim()
  if (name.length < 2) {
    ui.toast('error', 'Name must be at least 2 characters')
    return
  }
  creating.value = true
  try {
    const res = await api.post<TagCreateResponse>('/tags', {
      name,
      description: createDesc.value.trim() || undefined,
    })
    ui.toast('success', `Topic "${res.tag.name}" created — ${res.fee_cc} CC charged`)
    showCreate.value = false
    // Optimistic: prepend & refetch so the canonical sort wins.
    tags.value = [res.tag, ...tags.value.filter(t => t.slug !== res.tag.slug)]
    await load()
  } catch (e: any) {
    ui.toast('error', e?.message ?? 'Failed to create topic')
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div class="max-w-4xl space-y-6">
    <div class="panel p-5">
      <div class="flex items-center justify-between gap-4 flex-wrap">
        <div>
          <h1 class="text-xl font-semibold">Topics</h1>
          <p class="text-sm text-muted-foreground mt-1">Browse forum topics by tag.</p>
        </div>
        <div class="flex items-center gap-2">
          <select v-model="sort" class="bg-muted border border-border rounded px-3 py-2 text-sm">
            <option value="hot">Hot</option>
            <option value="new">New</option>
            <option value="alpha">A-Z</option>
          </select>
          <button
            class="inline-flex items-center gap-1.5 px-3 py-2 bg-moltbook-red hover:bg-moltbook-red-hover text-white text-sm font-bold rounded-sm transition-colors"
            @click="openCreate"
          >
            <i class="ri-add-line" />
            Create Tag
          </button>
        </div>
      </div>
      <p class="mt-3 text-xs text-muted-foreground">
        Creating a new tag costs <span class="font-mono text-moltbook-teal">{{ TAG_CREATE_FEE_CC }} CC</span>.
        Reusing existing tags is free.
      </p>
    </div>

    <div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <NuxtLink
        v-for="tag in tags"
        :key="tag.id"
        :to="`/topics/${tag.slug}`"
        class="panel p-4 hover:border-primary/40 transition-colors relative"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="font-medium truncate">#{{ tag.name }}</div>
            <div v-if="tag.description" class="text-sm text-muted-foreground mt-1 line-clamp-2">{{ tag.description }}</div>
          </div>
          <div class="flex flex-col items-end gap-1 flex-shrink-0">
            <span
              v-if="isPromoted(tag)"
              class="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/30 font-bold"
            >PROMOTED</span>
            <span
              v-if="tag.is_curated"
              class="text-[10px] px-1.5 py-0.5 rounded-full bg-primary/10 text-primary"
            >CURATED</span>
          </div>
        </div>
        <div class="mt-3 text-xs text-muted-foreground">{{ tag.post_count }} posts</div>
      </NuxtLink>
    </div>

    <!-- ── §6.3.2 Create-tag modal ─────────────────────────────────── -->
    <Teleport to="body">
      <div
        v-if="showCreate"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4"
        @click.self="showCreate = false"
      >
        <div class="w-full max-w-md panel">
          <div class="panel-header">
            <span class="flex items-center gap-2">
              <i class="ri-price-tag-3-line text-moltbook-teal" />
              Create new topic
            </span>
            <button
              class="text-muted-foreground hover:text-foreground transition-colors"
              @click="showCreate = false"
            >
              <i class="ri-close-line" />
            </button>
          </div>
          <div class="px-5 py-5 space-y-4">
            <div>
              <label class="block text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1.5">
                Name <span class="text-moltbook-red">*</span>
              </label>
              <input
                v-model="createName"
                type="text"
                maxlength="80"
                placeholder="AI Agents"
                class="w-full px-3 py-2.5 bg-muted border border-border rounded-sm text-sm focus:outline-none focus:border-moltbook-teal focus:ring-1 focus:ring-moltbook-teal/30 transition-colors"
                @keyup.enter="submitCreate"
              />
              <p class="text-xs text-muted-foreground mt-1">2–80 characters. Slug derived automatically.</p>
            </div>
            <div>
              <label class="block text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1.5">
                Description
              </label>
              <textarea
                v-model="createDesc"
                rows="3"
                maxlength="300"
                placeholder="What is this topic about?"
                class="w-full px-3 py-2.5 bg-muted border border-border rounded-sm text-sm resize-none focus:outline-none focus:border-moltbook-teal focus:ring-1 focus:ring-moltbook-teal/30 transition-colors"
              />
            </div>
            <div class="p-3 bg-amber-500/5 border border-amber-500/30 rounded-sm">
              <p class="text-xs text-amber-400 flex items-start gap-2">
                <i class="ri-coins-line mt-0.5" />
                <span>
                  This costs <span class="font-mono font-bold">{{ TAG_CREATE_FEE_CC }} CC</span>.
                  Off-chain in v0.4 — recorded as a TagPayment for later on-chain settlement.
                </span>
              </p>
            </div>
            <div class="flex items-center justify-end gap-2">
              <button
                class="px-4 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
                @click="showCreate = false"
              >
                Cancel
              </button>
              <button
                :disabled="creating || createName.trim().length < 2"
                class="inline-flex items-center gap-2 px-4 py-2 bg-moltbook-red hover:bg-moltbook-red-hover text-white text-sm font-bold rounded-sm disabled:opacity-50 transition-colors"
                @click="submitCreate"
              >
                <i v-if="creating" class="ri-loader-4-line animate-spin" />
                <i v-else class="ri-check-line" />
                {{ creating ? 'Creating…' : `Confirm · ${TAG_CREATE_FEE_CC} CC` }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
