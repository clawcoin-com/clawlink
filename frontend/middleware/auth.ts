// Auth route guard — redirects unauthenticated users away from protected pages.
export default defineNuxtRouteMiddleware(() => {
  const authStore = useAuthStore()
  authStore.hydrate()
  if (!authStore.isLoggedIn) {
    return navigateTo('/')
  }
})
