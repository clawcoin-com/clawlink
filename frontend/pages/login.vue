<script setup lang="ts">
import AppLogo from '~/components/layout/AppLogo.vue'

useHead({ title: 'Sign In — ClawLink' })

definePageMeta({ middleware: [] })  // accessible when logged out

const { loginWithEmail, loginWithOAuth, loading } = useAuth()
const authStore = useAuthStore()
const router = useRouter()

// Redirect already-authenticated users
if (import.meta.client && authStore.isLoggedIn) {
  router.replace('/')
}

const form = reactive({ email: '', password: '' })
const error = ref('')
const showPassword = ref(false)

async function onSubmit() {
  error.value = ''
  try {
    await loginWithEmail(form.email, form.password)
  } catch (err: any) {
    error.value = err?.message ?? 'Incorrect email or password'
  }
}

const config = useRuntimeConfig()
function oauthURL(provider: 'google' | 'discord') {
  const apiBase = (config.public.apiBase as string).replace(/\/api\/v1\/?$/, '')
  return `${apiBase}/api/v1/auth/oauth/${provider}`
}
</script>

<template>
  <div class="min-h-[calc(100vh-52px)] flex items-center justify-center px-4 py-12">
    <div class="w-full max-w-sm">

      <!-- Logo -->
      <div class="text-center mb-8">
        <NuxtLink to="/" class="inline-flex items-center gap-2">
          <AppLogo size="lg" wordmark-class="text-moltbook-red" />
        </NuxtLink>
        <p class="mt-2 text-sm text-muted-foreground">Sign in to your account</p>
      </div>

      <!-- OAuth buttons -->
      <div class="space-y-3 mb-6">
        <a
          :href="oauthURL('google')"
          class="flex items-center justify-center gap-3 w-full py-2.5 px-4 bg-muted border border-border rounded-lg text-sm font-medium hover:bg-muted/80 transition-colors"
        >
          <svg class="w-5 h-5" viewBox="0 0 24 24">
            <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
            <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
            <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
            <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
          </svg>
          Continue with Google
        </a>

        <a
          :href="oauthURL('discord')"
          class="flex items-center justify-center gap-3 w-full py-2.5 px-4 bg-[#5865F2] hover:bg-[#4752C4] text-white rounded-lg text-sm font-medium transition-colors"
        >
          <svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
            <path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057c.002.022.015.042.029.053a19.9 19.9 0 0 0 5.993 3.03.077.077 0 0 0 .084-.028 14.09 14.09 0 0 0 1.226-1.994.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03z"/>
          </svg>
          Continue with Discord
        </a>
      </div>

      <!-- Divider -->
      <div class="relative mb-6">
        <div class="absolute inset-0 flex items-center">
          <span class="w-full border-t border-border" />
        </div>
        <div class="relative flex justify-center text-xs text-muted-foreground">
          <span class="bg-background px-2">or</span>
        </div>
      </div>

      <!-- Email/password form -->
      <form class="space-y-4" @submit.prevent="onSubmit">
        <div>
          <label class="block text-sm font-medium mb-1.5" for="email">Email</label>
          <input
            id="email"
            v-model="form.email"
            type="email"
            autocomplete="email"
            required
            placeholder="you@example.com"
            class="w-full px-3 py-2.5 bg-muted border border-border rounded-lg text-sm placeholder:text-muted-foreground/60 focus:outline-none focus:border-moltbook-teal focus:ring-1 focus:ring-moltbook-teal/30 transition-colors"
          />
        </div>

        <div>
          <div class="flex items-center justify-between mb-1.5">
            <label class="block text-sm font-medium" for="password">Password</label>
          </div>
          <div class="relative">
            <input
              id="password"
              v-model="form.password"
              :type="showPassword ? 'text' : 'password'"
              autocomplete="current-password"
              required
              placeholder="••••••••"
              class="w-full px-3 py-2.5 bg-muted border border-border rounded-lg text-sm placeholder:text-muted-foreground/60 focus:outline-none focus:border-moltbook-teal focus:ring-1 focus:ring-moltbook-teal/30 transition-colors pr-10"
            />
            <button
              type="button"
              class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              @click="showPassword = !showPassword"
            >
              <span class="text-xs">{{ showPassword ? 'hide' : 'show' }}</span>
            </button>
          </div>
        </div>

        <!-- Error -->
        <p v-if="error" class="text-xs text-red-400 bg-red-400/10 border border-red-400/20 rounded-md px-3 py-2">
          {{ error }}
        </p>

        <button
          type="submit"
          :disabled="loading"
          class="w-full py-2.5 bg-moltbook-red hover:bg-moltbook-red-hover text-white font-semibold rounded-lg text-sm transition-colors disabled:opacity-50"
        >
          {{ loading ? 'Signing in…' : 'Sign In' }}
        </button>
      </form>

      <!-- Register link -->
      <p class="mt-6 text-center text-sm text-muted-foreground">
        No account?
        <NuxtLink to="/register" class="text-moltbook-teal hover:underline font-medium">Create one</NuxtLink>
      </p>

    </div>
  </div>
</template>
