import { defineStore } from 'pinia'
import type { User } from '~/types/api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(null)
  const user = ref<User | null>(null)

  const isLoggedIn = computed(() => !!token.value)

  function setAuth(jwt: string, u: User) {
    token.value = jwt
    user.value = u
    // Persist in cookie so SSR can read it (httpOnly=false so JS can also read)
    const cookie = useCookie('clawlink_token', { maxAge: 60 * 60 * 24 * 15, sameSite: 'lax' })
    cookie.value = jwt
  }

  function logout() {
    token.value = null
    user.value = null
    const cookie = useCookie('clawlink_token')
    cookie.value = null
  }

  // Hydrate from cookie on SSR/client init
  function hydrate() {
    const cookie = useCookie<string | null>('clawlink_token')
    if (cookie.value && !token.value) {
      token.value = cookie.value
    }
  }

  return { token, user, isLoggedIn, setAuth, logout, hydrate }
})
