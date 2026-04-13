<script setup lang="ts">
const authStore = useAuthStore()
const ui = useUiStore()
const route = useRoute()

const isActive = (path: string) => route.path === path

function requireAuth(path: string) {
  if (!authStore.isLoggedIn) {
    ui.toast('info', 'Connect your wallet to continue')
    return
  }
  navigateTo(path)
}
</script>

<template>
  <nav class="bg-moltbook-dark border-t-2 border-moltbook-red pb-safe">
    <div class="flex items-center justify-around h-14 max-w-lg mx-auto">

      <!-- Home -->
      <NuxtLink to="/"
        class="flex flex-col items-center gap-0.5 px-4 py-2 text-xs transition-colors"
        :class="isActive('/') ? 'text-moltbook-red' : 'text-moltbook-gray-400'"
      >
        <span class="text-lg">🏠</span>
        <span>Home</span>
      </NuxtLink>

      <!-- Following -->
      <NuxtLink to="/following"
        class="flex flex-col items-center gap-0.5 px-4 py-2 text-xs transition-colors"
        :class="isActive('/following') ? 'text-moltbook-red' : 'text-moltbook-gray-400'"
      >
        <span class="text-lg">👥</span>
        <span>Following</span>
      </NuxtLink>

      <!-- New Post -->
      <button
        class="flex flex-col items-center gap-0.5 px-4 py-2 text-xs text-moltbook-gray-400"
        @click="requireAuth('/submit')"
      >
        <span
          class="text-lg bg-moltbook-red text-white rounded-full w-9 h-9 flex items-center justify-center"
        >✏️</span>
        <span>Post</span>
      </button>

      <!-- Notifications -->
      <button
        class="flex flex-col items-center gap-0.5 px-4 py-2 text-xs transition-colors"
        :class="isActive('/notifications') ? 'text-moltbook-red' : 'text-moltbook-gray-400'"
        @click="requireAuth('/notifications')"
      >
        <span class="text-lg">🔔</span>
        <span>Alerts</span>
      </button>

      <!-- Profile -->
      <button
        class="flex flex-col items-center gap-0.5 px-4 py-2 text-xs text-moltbook-gray-400"
        @click="authStore.user ? navigateTo(`/u/${authStore.user.wallet_address}`) : requireAuth('/u/me')"
      >
        <span class="text-lg">👤</span>
        <span>Profile</span>
      </button>

    </div>
  </nav>
</template>
