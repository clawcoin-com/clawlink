<script setup lang="ts">
import AppLogo from '~/components/layout/AppLogo.vue'

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
        <div class="group-hover:scale-[1.03] transition-transform">
          <AppLogo size="md" wordmark-class="group-hover:text-glow-teal transition-all" />
        </div>
        <div class="flex items-baseline gap-2">
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
