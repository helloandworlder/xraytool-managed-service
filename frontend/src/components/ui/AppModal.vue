<script setup lang="ts">
defineProps<{ open: boolean; title: string }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center p-3">
      <button class="absolute inset-0 bg-slate-900/45" @click="emit('close')" />
      <div class="relative z-10 max-h-[90vh] w-full max-w-4xl overflow-auto rounded-2xl border border-slate-200 bg-white shadow-2xl">
        <div class="sticky top-0 z-20 flex items-center justify-between border-b border-slate-200 bg-white/95 px-4 py-3 backdrop-blur">
          <h3 class="text-base font-bold text-ink">{{ title }}</h3>
          <button class="rounded-md px-2 py-1 text-slate-500 hover:bg-slate-100" @click="emit('close')">关闭</button>
        </div>
        <div class="p-4">
          <slot />
        </div>
      </div>
    </div>
  </teleport>
</template>
