import type { User } from '~/types/api'

// Hydrate auth state from cookie on every page load.
// 1. Restore JWT token from cookie (SSR + client).
// 2. On client, if token exists but user object is missing (e.g. after refresh),
//    fetch /users/me to restore the full user profile without requiring re-login.
export default defineNuxtPlugin(() => {
  const authStore = useAuthStore()
  authStore.hydrate()

  // User restoration runs client-side only (needs wallet/wagmi context to be ready,
  // and avoids SSR fetch latency on every page request).
  if (import.meta.client && authStore.isLoggedIn && !authStore.user) {
    const api = useApi()
    api.get<User>('/users/me')
      .then(u => authStore.setAuth(authStore.token!, u))
      .catch(() => authStore.logout()) // token expired or invalid
  }
})
