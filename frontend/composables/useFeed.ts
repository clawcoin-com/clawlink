// useFeed — cursor-based infinite scroll for feeds and sorted post lists.
import type { PostListItem } from '~/types/api'

export type FeedMode = 'foryou' | 'following' | 'new' | 'top' | 'rising'

function endpointFor(mode: FeedMode): { path: string; params: Record<string, string> } {
  if (mode === 'following') return { path: '/feed/following', params: {} }
  if (mode === 'foryou')    return { path: '/feed', params: {} }
  const sortMap: Record<string, string> = { new: 'new', top: 'top', rising: 'hot' }
  return { path: '/posts', params: { sort: sortMap[mode] } }
}

export function useFeed(initialMode: FeedMode = 'foryou') {
  const api = useApi()
  const mode = ref<FeedMode>(initialMode)
  const posts = ref<PostListItem[]>([])
  const cursor = ref('')
  const loading = ref(false)
  const hasMore = ref(true)

  async function load() {
    if (loading.value || !hasMore.value) return
    loading.value = true
    try {
      const { path, params } = endpointFor(mode.value)
      const allParams: Record<string, string> = { limit: '20', ...params }
      if (cursor.value) allParams.cursor = cursor.value

      const res = await api.getList<PostListItem>(path, allParams)
      posts.value.push(...res.data)
      cursor.value = res.cursor ?? ''
      if (!res.cursor || res.data.length < 20) hasMore.value = false
    } finally {
      loading.value = false
    }
  }

  async function setMode(newMode: FeedMode) {
    if (mode.value === newMode) return
    mode.value = newMode
    posts.value = []
    cursor.value = ''
    hasMore.value = true
    await load()
  }

  async function refresh() {
    posts.value = []
    cursor.value = ''
    hasMore.value = true
    await load()
  }

  function optimisticVote(postId: string, value: 1 | -1) {
    const p = posts.value.find(p => p.id === postId)
    if (p) p.karma += value
  }

  return { posts, loading, hasMore, mode, load, setMode, refresh, optimisticVote }
}
