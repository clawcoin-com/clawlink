<script setup lang="ts">
import type { Tag } from '~/types/api'

const props = defineProps<{ modelValue: string[]; tags: Tag[]; max?: number }>()
const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()
const input = ref('')

const max = computed(() => props.max ?? 3)
const suggestions = computed(() => {
  const q = input.value.trim().toLowerCase()
  if (!q) return props.tags.slice(0, 8)
  return props.tags.filter(t => t.name.toLowerCase().includes(q)).slice(0, 8)
})

function add(name: string) {
  const trimmed = name.trim()
  if (!trimmed) return
  const exists = props.modelValue.some(v => v.toLowerCase() === trimmed.toLowerCase())
  if (exists || props.modelValue.length >= max.value) {
    input.value = ''
    return
  }
  emit('update:modelValue', [...props.modelValue, trimmed])
  input.value = ''
}
function remove(name: string) {
  emit('update:modelValue', props.modelValue.filter(v => v !== name))
}
</script>

<template>
  <div class="space-y-2">
    <div class="flex flex-wrap gap-2">
      <span v-for="tag in modelValue" :key="tag" class="px-2 py-1 rounded-full bg-muted text-xs flex items-center gap-1">
        #{{ tag }}
        <button type="button" @click="remove(tag)">×</button>
      </span>
    </div>
    <input
      v-model="input"
      type="text"
      :placeholder="modelValue.length >= max ? `Up to ${max} tags` : 'Add up to 3 tags'"
      class="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm"
      @keydown.enter.prevent="add(input)"
    />
    <div class="flex flex-wrap gap-2">
      <button
        v-for="tag in suggestions"
        :key="tag.id"
        type="button"
        class="px-2 py-1 rounded-full text-xs bg-accent/20 hover:bg-accent/30"
        @click="add(tag.name)"
      >#{{ tag.name }}</button>
    </div>
  </div>
</template>
