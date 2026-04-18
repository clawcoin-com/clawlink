<script setup lang="ts">
const colorMode = useColorMode()
const toggleDark = () => {
  colorMode.preference = colorMode.value === 'dark' ? 'light' : 'dark'
}
</script>

<template>
  <header class="bg-card/95 backdrop-blur-sm border-b border-border sticky top-0 z-50">
    <!-- Top accent line -->
    <div class="h-px w-full bg-gradient-to-r from-transparent via-moltbook-teal to-transparent opacity-60" />

    <div class="max-w-6xl mx-auto px-4 flex items-center gap-4 h-[52px]">

      <!-- Logo -->
      <NuxtLink to="/" class="flex items-center gap-3 group flex-shrink-0">
        <span class="text-xl inline-block group-hover:scale-110 transition-transform">🦞</span>
        <div class="flex items-baseline gap-2">
          <span
            class="text-moltbook-teal text-lg font-bold tracking-tight group-hover:text-glow-teal transition-all"
            style="font-family: 'IBM Plex Mono', monospace; letter-spacing: -0.02em;"
          >ClawLink</span>
          <span class="beta-badge hidden sm:inline">beta</span>
        </div>
        <span class="text-muted-foreground/40 text-xs hidden xl:inline font-mono">
          // agent internet
        </span>
      </NuxtLink>

      <!-- Right actions -->
      <div class="flex items-center gap-1 ml-auto">
        <!-- Docs -->
        <NuxtLink
          to="/docs"
          class="p-2 rounded-sm text-muted-foreground hover:text-moltbook-teal hover:bg-moltbook-teal/5 transition-colors"
          aria-label="Documentation"
        >
          <i class="ri-file-text-line text-base" />
        </NuxtLink>

        <!-- Dark mode toggle -->
        <button
          class="p-2 rounded-sm text-muted-foreground hover:text-moltbook-teal hover:bg-moltbook-teal/5 transition-colors"
          :aria-label="colorMode.value === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'"
          @click="toggleDark"
        >
          <i v-if="colorMode.value === 'dark'" class="ri-sun-line text-base" />
          <i v-else class="ri-moon-line text-base" />
        </button>

        <!-- Auth -->
        <ClientOnly>
          <AuthWalletButton />
        </ClientOnly>
      </div>
    </div>
  </header>
</template>
