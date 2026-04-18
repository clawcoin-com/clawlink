<script setup lang="ts">
// Handles JWT handoff from:
//   - Email verification redirect: /api/v1/auth/verify-email?token=... → this page?token=...
//   - OAuth callback:              /api/v1/auth/oauth/:provider/callback → this page?token=...
//   - Error case:                  this page?error=...

useHead({ title: 'Signing in… — ClawLink' })

const route = useRoute()
const authStore = useAuthStore()
const api = useApi()
const ui = useUiStore()

const status = ref<'loading' | 'success' | 'error'>('loading')
const errorMsg = ref('')

onMounted(async () => {
  const token = route.query.token as string | undefined
  const errorParam = route.query.error as string | undefined

  if (errorParam) {
    status.value = 'error'
    errorMsg.value = decodeURIComponent(errorParam)
    return
  }

  if (!token) {
    status.value = 'error'
    errorMsg.value = 'No token received'
    return
  }

  try {
    // Store token then fetch full user profile.
    // We set token first so useApi can inject it into the /users/me request.
    authStore.token = token
    const cookie = useCookie('clawlink_token', { maxAge: 60 * 60 * 24 * 15, sameSite: 'lax' })
    cookie.value = token

    const user = await api.get('/users/me')
    authStore.setAuth(token, user as any)

    status.value = 'success'
    ui.toast('success', `Welcome, ${(user as any).username}!`)

    await navigateTo('/')
  } catch (err: any) {
    status.value = 'error'
    errorMsg.value = err?.message ?? 'Authentication failed'
    authStore.logout()
  }
})
</script>

<template>
  <div class="min-h-[calc(100vh-52px)] flex items-center justify-center px-4">
    <div class="text-center space-y-4">

      <div v-if="status === 'loading'">
        <span class="text-4xl block mb-4 animate-pulse">🦞</span>
        <p class="text-sm text-muted-foreground">Signing you in…</p>
      </div>

      <div v-else-if="status === 'success'">
        <span class="text-4xl block mb-4">✅</span>
        <p class="text-sm text-muted-foreground">Redirecting…</p>
      </div>

      <div v-else class="space-y-3">
        <span class="text-4xl block">❌</span>
        <p class="text-sm font-medium">Sign-in failed</p>
        <p class="text-xs text-muted-foreground">{{ errorMsg }}</p>
        <NuxtLink
          to="/login"
          class="inline-block text-sm text-moltbook-teal hover:underline"
        >← Back to login</NuxtLink>
      </div>

    </div>
  </div>
</template>
