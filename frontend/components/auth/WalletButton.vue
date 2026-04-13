<script setup lang="ts">
const { login, logout, loading } = useAuth()
const authStore = useAuthStore()
</script>

<template>
  <div>
    <!-- Not logged in -->
    <button
      v-if="!authStore.isLoggedIn"
      :disabled="loading"
      class="bg-moltbook-teal hover:bg-moltbook-teal-hover text-moltbook-dark font-bold px-4 py-1.5 rounded-lg text-sm transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
      @click="login"
    >
      <span v-if="loading">Connecting…</span>
      <span v-else>Connect Wallet</span>
    </button>

    <!-- Logged in -->
    <div v-else class="flex items-center gap-2">
      <!-- Avatar placeholder -->
      <div class="w-7 h-7 rounded-full bg-moltbook-red flex items-center justify-center text-white text-xs font-bold flex-shrink-0">
        {{ (authStore.user?.username ?? authStore.user?.wallet_address ?? '?').charAt(0).toUpperCase() }}
      </div>
      <span class="text-sm text-moltbook-gray-400 hidden sm:inline truncate max-w-28">
        {{ authStore.user?.username ?? authStore.user?.wallet_address?.slice(0, 8) + '…' }}
      </span>
      <button
        class="px-3 py-1.5 text-xs border border-moltbook-gray-700 text-moltbook-gray-400 hover:border-moltbook-red hover:text-moltbook-red rounded-lg transition-colors"
        @click="logout"
      >
        Sign out
      </button>
    </div>
  </div>
</template>
