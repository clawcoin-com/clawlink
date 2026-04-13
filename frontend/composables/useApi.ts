// useApi — $fetch wrapper with auto auth injection and unified error handling.
import type { ApiResponse, ApiListResponse } from '~/types/api'

export function useApi() {
  const config = useRuntimeConfig()
  const authStore = useAuthStore()

  // SSR: use the private server-only URL (Docker internal network).
  // Browser: use the public URL (goes through the nginx proxy).
  const baseURL = import.meta.server
    ? (config.apiBase || config.public.apiBase)
    : config.public.apiBase

  const apiFetch = $fetch.create({
    baseURL,
    onRequest({ options }) {
      if (authStore.token) {
        options.headers = {
          ...options.headers,
          Authorization: `Bearer ${authStore.token}`,
        }
      }
    },
    onResponseError({ response }) {
      if (response.status === 401) {
        authStore.logout()
        navigateTo('/')
      }
    },
  })

  async function get<T>(path: string, params?: Record<string, string | number | undefined>): Promise<T> {
    const query: Record<string, string> = {}
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        if (v !== undefined) query[k] = String(v)
      }
    }
    const res = await apiFetch<ApiResponse<T>>(path, { method: 'GET', params: query })
    if (!res.success) throw new Error(res.error?.message ?? 'API error')
    return res.data
  }

  async function post<T>(path: string, body?: unknown): Promise<T> {
    const res = await apiFetch<ApiResponse<T>>(path, { method: 'POST', body })
    if (!res.success) throw new Error(res.error?.message ?? 'API error')
    return res.data
  }

  async function put<T>(path: string, body?: unknown): Promise<T> {
    const res = await apiFetch<ApiResponse<T>>(path, { method: 'PUT', body })
    if (!res.success) throw new Error(res.error?.message ?? 'API error')
    return res.data
  }

  async function del<T>(path: string): Promise<T> {
    const res = await apiFetch<ApiResponse<T>>(path, { method: 'DELETE' })
    if (!res.success) throw new Error(res.error?.message ?? 'API error')
    return res.data
  }

  async function getList<T>(path: string, params?: Record<string, string | number | undefined>): Promise<{ data: T[]; cursor: string }> {
    const query: Record<string, string> = {}
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        if (v !== undefined) query[k] = String(v)
      }
    }
    const res = await apiFetch<ApiListResponse<T>>(path, { method: 'GET', params: query })
    if (!res.success) throw new Error(res.error?.message ?? 'API error')
    return { data: res.data, cursor: res.meta?.cursor ?? '' }
  }

  return { get, post, put, del, getList }
}
