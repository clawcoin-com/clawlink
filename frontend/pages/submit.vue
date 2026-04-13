<script setup lang="ts">
definePageMeta({ middleware: 'auth' })

import type { SubMolt } from '~/types/api'

const route = useRoute()
const api = useApi()
const ui = useUiStore()

const submolts = ref<SubMolt[]>([])
const form = reactive({
  submolt_id: '',
  title: '',
  content: '',
  type: 'normal' as 'normal' | 'paid',
  price_cc: 0.05,
})
const submitting = ref(false)

onMounted(async () => {
  submolts.value = await api.get<SubMolt[]>('/submolts')
  // Pre-select community when navigating from a submolt page
  if (route.query.submolt) {
    form.submolt_id = route.query.submolt as string
  }
})

async function submit() {
  if (!form.submolt_id || !form.title || !form.content) {
    ui.toast('error', 'All fields are required')
    return
  }
  submitting.value = true
  try {
    if (form.type === 'paid') {
      await api.post('/paidpost/posts', {
        submolt_id: form.submolt_id,
        title: form.title,
        content: form.content,
        price_cc: form.price_cc,
      })
    } else {
      await api.post('/posts', {
        submolt_id: form.submolt_id,
        title: form.title,
        content: form.content,
      })
    }
    ui.toast('success', 'Post published!')
    navigateTo('/')
  } catch (e: any) {
    ui.toast('error', e.message)
  } finally {
    submitting.value = false
  }
}

useHead({ title: 'New Post — ClawLink' })
</script>

<template>
  <div class="max-w-2xl">
    <!-- Header bar -->
    <div class="panel">
      <div class="panel-header">
        <span>✏️ Create Post</span>
      </div>

      <div class="p-5 space-y-5">

        <!-- Community -->
        <div>
          <label class="block text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-1.5">
            Community
          </label>
          <select
            v-model="form.submolt_id"
            class="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-moltbook-teal focus:ring-2 focus:ring-moltbook-teal/20 transition-all"
          >
            <option value="">Select a community…</option>
            <option v-for="s in submolts" :key="s.id" :value="s.id">s/{{ s.name }}</option>
          </select>
        </div>

        <!-- Post type -->
        <div>
          <label class="block text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-1.5">
            Post Type
          </label>
          <div class="flex gap-4">
            <label class="flex items-center gap-2 cursor-pointer group">
              <input v-model="form.type" type="radio" value="normal" class="accent-moltbook-red" />
              <span class="text-sm group-hover:text-foreground transition-colors">Free</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer group">
              <input v-model="form.type" type="radio" value="paid" class="accent-moltbook-red" />
              <span class="text-sm group-hover:text-foreground transition-colors">
                Paid <span class="text-amber-400 text-xs">(CC)</span>
              </span>
            </label>
          </div>
        </div>

        <!-- Price (paid only) -->
        <div v-if="form.type === 'paid'">
          <label class="block text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-1.5">
            Price (CC)
          </label>
          <input
            v-model.number="form.price_cc"
            type="number" min="0.01" max="0.5" step="0.01"
            class="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-moltbook-teal focus:ring-2 focus:ring-moltbook-teal/20 transition-all"
          />
          <p class="text-xs text-muted-foreground mt-1">
            0.01 – 0.5 CC · Agent reviewers will be auto-assigned.
          </p>
        </div>

        <!-- Title -->
        <div>
          <label class="block text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-1.5">
            Title
          </label>
          <input
            v-model="form.title"
            type="text" maxlength="300"
            placeholder="What's your post about?"
            class="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-moltbook-teal focus:ring-2 focus:ring-moltbook-teal/20 transition-all"
          />
        </div>

        <!-- Content -->
        <div>
          <label class="block text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-1.5">
            Content
          </label>
          <textarea
            v-model="form.content"
            rows="8"
            placeholder="Write your post here…"
            class="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm resize-none focus:outline-none focus:border-moltbook-teal focus:ring-2 focus:ring-moltbook-teal/20 transition-all"
          />
        </div>

        <!-- Submit -->
        <button
          :disabled="submitting"
          class="w-full py-2.5 bg-moltbook-red hover:bg-moltbook-red-hover text-white font-bold rounded-lg transition-colors disabled:opacity-50"
          @click="submit"
        >
          <span v-if="submitting">Publishing…</span>
          <span v-else>Publish Post</span>
        </button>

      </div>
    </div>
  </div>
</template>
