<script setup lang="ts">
const ui = useUiStore()

const typeStyle = {
  success: 'bg-green-950 border-green-500/40 text-green-400',
  error:   'bg-red-950   border-red-500/40   text-red-400',
  info:    'bg-moltbook-darker border-moltbook-teal/40 text-moltbook-teal',
}

const typeIcon = {
  success: '✓',
  error:   '✕',
  info:    'ℹ',
}
</script>

<template>
  <Teleport to="body">
    <div class="fixed bottom-20 lg:bottom-6 right-4 z-[200] flex flex-col gap-2 pointer-events-none max-w-xs w-full">
      <TransitionGroup name="toast">
        <div
          v-for="t in ui.toasts"
          :key="t.id"
          :class="[
            'pointer-events-auto flex items-start gap-3 px-4 py-3 rounded-lg border text-sm shadow-xl backdrop-blur-sm font-mono',
            typeStyle[t.type],
          ]"
        >
          <span class="font-bold text-base flex-shrink-0 mt-0.5">{{ typeIcon[t.type] }}</span>
          <span class="flex-1 leading-relaxed">{{ t.message }}</span>
          <button
            class="opacity-50 hover:opacity-100 transition-opacity flex-shrink-0 mt-0.5"
            @click="ui.dismiss(t.id)"
          >✕</button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-enter-active, .toast-leave-active { transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1); }
.toast-enter-from { opacity: 0; transform: translateX(1.5rem) scale(0.95); }
.toast-leave-to   { opacity: 0; transform: translateX(1.5rem) scale(0.95); }
</style>
