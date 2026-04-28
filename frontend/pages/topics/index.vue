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
</script>

<template>
  <div class="max-w-4xl space-y-6">
    <div class="panel p-5">
      <div class="flex items-center justify-between gap-4">
        <div>
          <h1 class="text-xl font-semibold">Topics</h1>
          <p class="text-sm text-muted-foreground mt-1">Browse forum topics by tag.</p>
        </div>
        <select v-model="sort" class="bg-muted border border-border rounded px-3 py-2 text-sm">
          <option value="hot">Hot</option>
          <option value="new">New</option>
          <option value="alpha">A-Z</option>
        </select>
      </div>
    </div>

    <div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <NuxtLink v-for="tag in tags" :key="tag.id" :to="`/topics/${tag.slug}`" class="panel p-4 hover:border-primary/40 transition-colors">
        <div class="flex items-start justify-between gap-3">
          <div>
            <div class="font-medium">#{{ tag.name }}</div>
            <div v-if="tag.description" class="text-sm text-muted-foreground mt-1 line-clamp-2">{{ tag.description }}</div>
          </div>
          <span v-if="tag.is_curated" class="text-[10px] px-1.5 py-0.5 rounded-full bg-primary/10 text-primary">CURATED</span>
        </div>
        <div class="mt-3 text-xs text-muted-foreground">{{ tag.post_count }} posts</div>
      </NuxtLink>
    </div>
  </div>
</template>
