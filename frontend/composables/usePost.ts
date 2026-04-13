// usePost — post detail, replies, voting, and reply submission.
import type { Post, Reply } from '~/types/api'

export function usePost(postId: string) {
  const api = useApi()
  const authStore = useAuthStore()
  const ui = useUiStore()

  const post = ref<Post | null>(null)
  const replies = ref<Reply[]>([])
  const loading = ref(true)

  async function fetch() {
    loading.value = true
    try {
      const [p, r] = await Promise.all([
        api.get<Post>(`/posts/${postId}`),
        api.get<Reply[]>(`/posts/${postId}/replies`),
      ])
      post.value = p
      replies.value = r ?? []
    } catch {
      // post stays null → template shows "not found"
    } finally {
      loading.value = false
    }
  }

  async function vote(value: 1 | -1) {
    if (!authStore.isLoggedIn) { ui.toast('error', 'Login required'); return }
    if (post.value) post.value.karma += value  // optimistic
    try {
      await api.post(`/posts/${postId}/vote`, { value })
    } catch {
      if (post.value) post.value.karma -= value  // rollback
    }
  }

  async function submitReply(content: string, parentId?: string) {
    if (!authStore.isLoggedIn) { ui.toast('error', 'Login required'); return }
    const reply = await api.post<Reply>(`/posts/${postId}/replies`, { content, parent_id: parentId ?? null })
    // Insert into tree
    if (parentId) {
      const parent = replies.value.find(r => r.id === parentId)
      if (parent) {
        parent.children = parent.children ?? []
        parent.children.push(reply)
      }
    } else {
      replies.value.unshift(reply)
    }
    ui.toast('success', 'Reply posted')
  }

  return { post, replies, loading, fetch, vote, submitReply }
}
