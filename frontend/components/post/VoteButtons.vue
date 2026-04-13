<script setup lang="ts">
const props = defineProps<{ postId: string; karma: number; vertical?: boolean }>()
const emit = defineEmits<{ vote: [value: 1 | -1] }>()

const authStore = useAuthStore()
const ui = useUiStore()
const voted = ref<1 | -1 | null>(null)

function vote(value: 1 | -1) {
  if (!authStore.isLoggedIn) { ui.toast('info', 'Login required to vote'); return }
  if (voted.value === value) return
  emit('vote', value)
  voted.value = value
}
</script>

<template>
  <div :class="['flex items-center gap-1', vertical ? 'flex-col' : 'flex-row']">
    <!-- Upvote -->
    <button
      :class="['vote-btn vote-btn-up', voted === 1 && 'active']"
      aria-label="Upvote"
      @click.stop="vote(1)"
    >
      <!-- Arrow Big Up (inline SVG) -->
      <svg
        :class="['w-6 h-6 transition-transform hover:scale-110', voted === 1 ? 'fill-current' : '']"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M9 18V12H5.5a.5.5 0 0 1-.354-.854l6.5-6.5a.5.5 0 0 1 .707 0l6.5 6.5A.5.5 0 0 1 18.5 12H15v6a1 1 0 0 1-1 1h-4a1 1 0 0 1-1-1z"
          :fill="voted === 1 ? 'currentColor' : 'none'"
        />
      </svg>
    </button>

    <!-- Score -->
    <span
      :class="[
        'karma tabular-nums font-medium text-sm min-w-[2ch] text-center',
        karma > 0 ? 'karma-positive' : karma < 0 ? 'karma-negative' : 'text-muted-foreground',
      ]"
    >{{ karma }}</span>

    <!-- Downvote -->
    <button
      :class="['vote-btn vote-btn-down', voted === -1 && 'active']"
      aria-label="Downvote"
      @click.stop="vote(-1)"
    >
      <svg
        :class="['w-6 h-6 transition-transform hover:scale-110', voted === -1 ? 'fill-current' : '']"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M15 6v6h3.5a.5.5 0 0 1 .354.854l-6.5 6.5a.5.5 0 0 1-.707 0l-6.5-6.5A.5.5 0 0 1 5.5 12H9V6a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1z"
          :fill="voted === -1 ? 'currentColor' : 'none'"
        />
      </svg>
    </button>
  </div>
</template>
