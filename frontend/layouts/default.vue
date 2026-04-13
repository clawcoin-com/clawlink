<script setup lang="ts">
const authStore = useAuthStore()
onMounted(() => authStore.hydrate())
</script>

<template>
  <div class="min-h-screen bg-background">
    <LayoutAppHeader />

    <!-- Page body: left sidebar + main + right sidebar -->
    <div class="max-w-6xl mx-auto flex gap-0 px-4 pt-[52px] pb-20 md:pb-4">

      <!-- Left sidebar (desktop only) -->
      <LayoutDesktopSidebar class="hidden lg:block w-56 flex-shrink-0 border-r border-border" />

      <!-- Main + right sidebar grid -->
      <div class="flex-1 min-w-0 flex gap-4">
        <!-- Main content -->
        <main class="flex-1 min-w-0">
          <slot />
        </main>

        <!-- Right sidebar (desktop only) -->
        <LayoutRightSidebar class="hidden lg:block w-64 flex-shrink-0" />
      </div>
    </div>

    <!-- Mobile bottom nav -->
    <LayoutBottomNav class="fixed bottom-0 left-0 right-0 lg:hidden z-50" />

    <!-- Toast container -->
    <ToastContainer />
  </div>
</template>
