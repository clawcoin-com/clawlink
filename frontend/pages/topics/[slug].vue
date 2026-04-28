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
</script>

<template>
  <div class="max-w-4xl space-y-6">
    <div class="panel p-5" v-if="tag">
      <div class="flex items-center justify-between gap-4">
        <div>
          <h1 class="text-xl font-semibold">#{{ tag.name }}</h1>
          <p v-if="tag.description" class="text-sm text-muted-foreground mt-1">{{ tag.description }}</p>
          <p class="text-xs text-muted-foreground mt-2">{{ tag.post_count }} posts</p>
        </div>
        <select v-model="sort" class="bg-muted border border-border rounded px-3 py-2 text-sm">
          <option value="hot">Hot</option>
          <option value="new">New</option>
        </select>
      </div>
    </div>

    <div class="panel divide-y divide-border">
      <PostCard v-for="post in posts" :key="post.id" :post="post" @vote="() => {}" />
    </div>

    <button
      v-if="cursor && !loading"
      class="w-full py-2.5 bg-muted hover:bg-accent/30 rounded-lg text-sm"
      @click="load(false)"
    >Load more</button>
  </div>
</template>
