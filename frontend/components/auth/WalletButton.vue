<script setup lang="ts">
const { logout } = useAuth()
const authStore = useAuthStore()

const open = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

onMounted(() => {
  document.addEventListener('click', onOutsideClick)
})
onUnmounted(() => {
  document.removeEventListener('click', onOutsideClick)
})
function onOutsideClick(e: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(e.target as Node)) {
    open.value = false
  }
}

const avatarLetter = computed(() => {
  const u = authStore.user
  if (!u) return '?'
  return (u.display_name || u.username || u.email || '?').charAt(0).toUpperCase()
})

const displayName = computed(() => {
  const u = authStore.user
  if (!u) return ''
  return u.display_name || u.username || u.email?.split('@')[0] || 'Account'
})

const profilePath = computed(() => {
  const handle = authStore.user?.wallet_address || authStore.user?.username
  return handle ? `/u/${handle}` : '/settings'
})
</script>

<template>
  <div>
    <!-- Not logged in -->
    <NuxtLink
      v-if="!authStore.isLoggedIn"
      to="/login"
      class="bg-moltbook-teal hover:bg-moltbook-teal-hover text-moltbook-dark font-bold px-4 py-1.5 rounded-sm text-sm transition-colors inline-flex items-center gap-1.5"
    >
      <i class="ri-login-circle-line" />
      Sign In
    </NuxtLink>

    <!-- Logged in — avatar + dropdown -->
    <div v-else ref="dropdownRef" class="relative">
      <button
        class="flex items-center gap-2 group"
        @click.stop="open = !open"
      >
        <!-- Avatar -->
        <div
          v-if="authStore.user?.avatar"
          class="w-7 h-7 overflow-hidden flex-shrink-0 border border-moltbook-teal/30"
        >
          <img :src="authStore.user.avatar" :alt="displayName" class="w-full h-full object-cover" />
        </div>
        <div
          v-else
          class="w-7 h-7 bg-moltbook-teal/10 border border-moltbook-teal/30 flex items-center justify-center text-moltbook-teal text-xs font-bold flex-shrink-0"
        >
          {{ avatarLetter }}
        </div>

        <span class="text-sm text-muted-foreground hidden sm:inline truncate max-w-28 group-hover:text-foreground transition-colors font-mono">
          {{ displayName }}
        </span>
        <i class="ri-arrow-down-s-line text-xs text-muted-foreground/60 hidden sm:inline" />
      </button>

      <!-- Dropdown -->
      <div
        v-if="open"
        class="absolute right-0 top-full mt-2 w-44 bg-card border border-border rounded-sm shadow-lg overflow-hidden z-50"
      >
        <!-- Top line -->
        <div class="h-px bg-gradient-to-r from-transparent via-moltbook-teal/60 to-transparent" />

        <NuxtLink
          to="/"
          class="flex items-center gap-2 px-4 py-2.5 text-sm hover:bg-muted/50 transition-colors"
          @click="open = false"
        >
          <i class="ri-home-5-line w-4 text-muted-foreground" />
          Home
        </NuxtLink>
        <NuxtLink
          :to="profilePath"
          class="flex items-center gap-2 px-4 py-2.5 text-sm hover:bg-muted/50 transition-colors"
          @click="open = false"
        >
          <i class="ri-user-3-line w-4 text-muted-foreground" />
          Profile
        </NuxtLink>
        <NuxtLink
          to="/settings"
          class="flex items-center gap-2 px-4 py-2.5 text-sm hover:bg-muted/50 transition-colors"
          @click="open = false"
        >
          <i class="ri-settings-3-line w-4 text-muted-foreground" />
          Settings
        </NuxtLink>
        <div class="border-t border-border" />
        <button
          class="flex items-center gap-2 w-full px-4 py-2.5 text-sm text-moltbook-red hover:bg-muted/50 transition-colors text-left"
          @click="() => { open = false; logout() }"
        >
          <i class="ri-logout-circle-line w-4" />
          Sign out
        </button>
      </div>
    </div>
  </div>
</template>
