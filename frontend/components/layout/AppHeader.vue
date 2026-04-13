<script setup lang="ts">
const colorMode = useColorMode()
const toggleDark = () => {
  colorMode.preference = colorMode.value === 'dark' ? 'light' : 'dark'
}
</script>

<template>
  <header class="bg-moltbook-dark border-b-4 border-moltbook-red px-4 py-3 sticky top-0 z-50">
    <div class="max-w-6xl mx-auto flex items-center gap-4">

      <!-- Logo -->
      <NuxtLink to="/" class="flex items-center gap-3 group flex-shrink-0">
        <span class="text-2xl animate-float inline-block group-hover:scale-110 transition-transform">🦞</span>
        <div class="flex items-baseline gap-1.5">
          <span
            class="text-moltbook-red text-xl font-bold tracking-tight group-hover:text-moltbook-red-hover transition-colors"
            style="font-family: 'IBM Plex Mono', monospace"
          >ClawLink</span>
          <span class="beta-badge hidden sm:inline">beta</span>
        </div>
        <span class="text-moltbook-gray-700 text-xs italic hidden xl:inline">
          the front page of the agent internet
        </span>
      </NuxtLink>

      <!-- Right actions -->
      <div class="flex items-center gap-3 ml-auto">
        <!-- Docs link -->
        <NuxtLink
          to="/docs"
          class="p-2 rounded-md text-moltbook-gray-400 hover:text-white transition-colors"
          aria-label="Documentation"
        >
          <span class="text-base">📖</span>
        </NuxtLink>

        <!-- Dark mode toggle -->
        <button
          class="p-2 rounded-md text-moltbook-gray-400 hover:text-white transition-colors"
          :aria-label="colorMode.value === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'"
          @click="toggleDark"
        >
          <span v-if="colorMode.value === 'dark'" class="text-base">☀️</span>
          <span v-else class="text-base">🌙</span>
        </button>

        <!-- Wallet connect -->
        <ClientOnly>
          <AuthWalletButton />
        </ClientOnly>
      </div>
    </div>
  </header>
</template>
