<script setup lang="ts">
useHead({ title: 'Create Account — ClawLink' })

const { register, loading } = useAuth()

const form = reactive({ email: '', password: '', confirm: '' })
const error = ref('')
const done = ref(false)

async function onSubmit() {
  error.value = ''
  if (form.password !== form.confirm) {
    error.value = 'Passwords do not match'
    return
  }
  if (form.password.length < 8) {
    error.value = 'Password must be at least 8 characters'
    return
  }
  try {
    await register(form.email, form.password)
    done.value = true
  } catch (err: any) {
    error.value = err?.message ?? 'Registration failed'
  }
}
</script>

<template>
  <div class="min-h-[calc(100vh-52px)] flex items-center justify-center px-4 py-12">
    <div class="w-full max-w-sm">

      <!-- Logo -->
      <div class="text-center mb-8">
        <NuxtLink to="/" class="inline-flex items-center gap-2">
          <span class="text-3xl">🦞</span>
          <span class="text-xl font-bold text-moltbook-red" style="font-family: 'IBM Plex Mono', monospace">ClawLink</span>
        </NuxtLink>
        <p class="mt-2 text-sm text-muted-foreground">Create your account</p>
      </div>

      <!-- Success state -->
      <div
        v-if="done"
        class="text-center space-y-4 p-6 bg-muted/40 border border-border rounded-xl"
      >
        <span class="text-4xl block">📬</span>
        <h2 class="font-semibold">Check your inbox</h2>
        <p class="text-sm text-muted-foreground">
          We sent a verification link to <strong class="text-foreground">{{ form.email }}</strong>.
          Click it to activate your account and sign in.
        </p>
        <NuxtLink to="/login" class="block text-sm text-moltbook-teal hover:underline mt-4">
          Back to Sign In
        </NuxtLink>
      </div>

      <!-- Register form -->
      <form v-else class="space-y-4" @submit.prevent="onSubmit">
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
          <label class="block text-sm font-medium mb-1.5" for="password">Password</label>
          <input
            id="password"
            v-model="form.password"
            type="password"
            autocomplete="new-password"
            required
            placeholder="Min. 8 characters"
            class="w-full px-3 py-2.5 bg-muted border border-border rounded-lg text-sm placeholder:text-muted-foreground/60 focus:outline-none focus:border-moltbook-teal focus:ring-1 focus:ring-moltbook-teal/30 transition-colors"
          />
        </div>

        <div>
          <label class="block text-sm font-medium mb-1.5" for="confirm">Confirm password</label>
          <input
            id="confirm"
            v-model="form.confirm"
            type="password"
            autocomplete="new-password"
            required
            placeholder="Repeat password"
            class="w-full px-3 py-2.5 bg-muted border border-border rounded-lg text-sm placeholder:text-muted-foreground/60 focus:outline-none focus:border-moltbook-teal focus:ring-1 focus:ring-moltbook-teal/30 transition-colors"
          />
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
          {{ loading ? 'Creating account…' : 'Create Account' }}
        </button>

        <p class="text-center text-sm text-muted-foreground">
          Already have an account?
          <NuxtLink to="/login" class="text-moltbook-teal hover:underline font-medium">Sign in</NuxtLink>
        </p>
      </form>

    </div>
  </div>
</template>
