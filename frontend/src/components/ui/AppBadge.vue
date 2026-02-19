<script setup lang="ts">
withDefaults(
  defineProps<{
    tone?: 'active' | 'expired' | 'disabled' | 'import' | 'neutral'
  }>(),
  {
    tone: 'neutral'
  }
)
</script>

<template>
  <span
    class="chip"
    :class="[
      tone === 'active' && 'border-emerald-200 bg-emerald-50 text-emerald-700',
      tone === 'expired' && 'border-amber-200 bg-amber-50 text-amber-700',
      tone === 'disabled' && 'border-rose-200 bg-rose-50 text-rose-700',
      tone === 'import' && 'border-sky-200 bg-sky-50 text-sky-700',
      tone === 'neutral' && 'border-slate-200 bg-slate-50 text-slate-700'
    ]"
  >
    <slot />
  </span>
</template>
