import { defineStore } from 'pinia'

export interface Toast {
  id: string
  type: 'success' | 'error' | 'info'
  message: string
}

export const useUiStore = defineStore('ui', () => {
  const toasts = ref<Toast[]>([])

  function toast(type: Toast['type'], message: string) {
    const id = Math.random().toString(36).slice(2)
    toasts.value.push({ id, type, message })
    setTimeout(() => dismiss(id), 4000)
  }

  function dismiss(id: string) {
    toasts.value = toasts.value.filter(t => t.id !== id)
  }

  return { toasts, toast, dismiss }
})
