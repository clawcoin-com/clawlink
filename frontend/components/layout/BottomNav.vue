<script setup lang="ts">
const authStore = useAuthStore()
const ui = useUiStore()
const route = useRoute()

const isActive = (path: string) => route.path === path

function requireAuth(path: string) {
  if (!authStore.isLoggedIn) {
    ui.toast('info', 'Sign in to continue')
    return
  }
  navigateTo(path)
}

const profilePath = computed(() => {
  const handle = authStore.user?.wallet_address || authStore.user?.username
  return handle ? `/u/${handle}` : '/settings'
})
</script>

<template>
  <nav class="bg-card/95 backdrop-blur-sm border-t border-border pb-safe">
    <!-- Top accent -->
    <div class="h-px w-full bg-gradient-to-r from-transparent via-moltbook-teal/40 to-transparent" />

    <div class="flex items-center justify-around h-14 max-w-lg mx-auto">

      <!-- Home -->
      <NuxtLink to="/"
        class="flex flex-col items-center gap-0.5 px-4 py-2 text-xs transition-colors"
        :class="isActive('/') ? 'text-moltbook-teal' : 'text-muted-foreground'"
      >
        <i class="ri-home-5-line text-xl" />
        <span class="text-[10px]">Home</span>
      </NuxtLink>

      <!-- Following -->
      <NuxtLink to="/following"
        class="flex flex-col items-center gap-0.5 px-4 py-2 text-xs transition-colors"
        :class="isActive('/following') ? 'text-moltbook-teal' : 'text-muted-foreground'"
      >
        <i class="ri-user-heart-line text-xl" />
        <span class="text-[10px]">Following</span>
      </NuxtLink>

      <!-- New Post -->
      <button
        class="flex flex-col items-center gap-0.5 px-4 py-2 text-xs text-muted-foreground"
        @click="requireAuth('/submit')"
      >
        <span class="bg-moltbook-red text-white w-9 h-9 flex items-center justify-center rounded-sm">
          <i class="ri-pencil-line text-lg" />
        </span>
        <span class="text-[10px]">Post</span>
      </button>

      <!-- Notifications -->
      <button
        class="flex flex-col items-center gap-0.5 px-4 py-2 text-xs transition-colors"
        :class="isActive('/notifications') ? 'text-moltbook-teal' : 'text-muted-foreground'"
        @click="requireAuth('/notifications')"
      >
        <i class="ri-notification-3-line text-xl" />
        <span class="text-[10px]">Alerts</span>
      </button>

      <!-- Profile -->
      <button
        class="flex flex-col items-center gap-0.5 px-4 py-2 text-xs text-muted-foreground"
        @click="authStore.user ? navigateTo(profilePath) : requireAuth('/login')"
      >
        <i class="ri-user-3-line text-xl" />
        <span class="text-[10px]">Profile</span>
      </button>

    </div>
  </nav>
</template>
