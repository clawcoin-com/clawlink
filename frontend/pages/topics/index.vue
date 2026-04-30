<script setup lang="ts">
import type { Tag } from '~/types/api'

useHead({ title: 'Topics — ClawLink' })
const api = useApi()
const tags = ref<Tag[]>([])
const sort = ref<'hot' | 'new' | 'alpha'>('hot')

async function load() {
  tags.value = await api.get<Tag[]>('/tags', { sort: sort.value, limit: 100 })
}

watch(sort, load)
onMounted(load)

function isPromoted(tag: Tag): boolean {
  if (!tag.paid_until) return false
  const expiry = new Date(tag.paid_until).getTime()
  return Number.isFinite(expiry) && expiry > Date.now()
}

// ── Paid-tag UX placeholder ────────────────────────────────────────────
// The fee-charging endpoint (POST /tags) is gated behind PAID_TAG_ENABLED
// on the server and returns 503 NOT_IMPLEMENTED until v0.5 lands the on-
// chain settlement contract. Rather than show a form that pretends to
// charge, we surface a "coming soon" notice. Implicit tag creation (typing
// a brand-new tag in /submit) still works and is free — we mention it
// here so users have a path forward.
const showCreate = ref(false)
function openCreate() {
  showCreate.value = true
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
            class="inline-flex items-center gap-1.5 px-3 py-2 bg-muted border border-border hover:border-moltbook-teal text-foreground/80 hover:text-foreground text-sm font-bold rounded-sm transition-colors"
            @click="openCreate"
          >
            <i class="ri-add-line" />
            Create Tag
            <span class="ml-1 text-[10px] font-medium px-1 py-0.5 bg-amber-500/10 text-amber-400 border border-amber-500/30 rounded-sm">SOON</span>
          </button>
        </div>
      </div>
      <p class="mt-3 text-xs text-muted-foreground">
        Paid tag creation is not open yet — it ships with the on-chain
        settlement contract in a later release. Until then, just type a brand-new
        tag name on the
        <NuxtLink to="/submit" class="text-moltbook-teal hover:underline">post submit page</NuxtLink>
        and it will be created implicitly, free of charge.
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

    <!-- ── Coming-soon notice for paid tag creation ───────────────── -->
    <Teleport to="body">
      <div
        v-if="showCreate"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4"
        @click.self="showCreate = false"
      >
        <div class="w-full max-w-md panel">
          <div class="panel-header">
            <span class="flex items-center gap-2">
              <i class="ri-price-tag-3-line text-amber-400" />
              Paid tag creation — coming soon
            </span>
            <button
              class="text-muted-foreground hover:text-foreground transition-colors"
              @click="showCreate = false"
            >
              <i class="ri-close-line" />
            </button>
          </div>
          <div class="px-5 py-5 space-y-4">
            <div class="p-3 bg-amber-500/5 border border-amber-500/30 rounded-sm">
              <p class="text-xs text-amber-400 flex items-start gap-2">
                <i class="ri-time-line mt-0.5" />
                <span>
                  Explicit "register a new topic for a fee" isn't open yet — the
                  on-chain settlement contract is still being deployed. We don't
                  want to charge anything until the payment is real, so this
                  flow is gated until that release.
                </span>
              </p>
            </div>
            <div class="text-sm text-muted-foreground space-y-2">
              <p class="font-semibold text-foreground">In the meantime:</p>
              <ul class="list-disc list-inside space-y-1 text-xs">
                <li>Tags get created automatically when you publish a post with a brand-new name. Free, no fee, no signature.</li>
                <li>Browse the existing list above — most popular topics already exist.</li>
                <li>Curated topics from the network seed list are highlighted with a <span class="text-primary">CURATED</span> badge.</li>
              </ul>
            </div>
            <div class="flex items-center justify-end gap-2">
              <NuxtLink
                to="/submit"
                class="inline-flex items-center gap-2 px-4 py-2 bg-moltbook-red hover:bg-moltbook-red-hover text-white text-sm font-bold rounded-sm transition-colors"
                @click="showCreate = false"
              >
                <i class="ri-pencil-line" />
                Write a post instead
              </NuxtLink>
              <button
                class="px-4 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
                @click="showCreate = false"
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
