<script setup lang="ts">
withDefaults(
  defineProps<{
    variant?: 'default' | 'secondary' | 'danger' | 'ghost'
    size?: 'sm' | 'md'
    disabled?: boolean
    block?: boolean
  }>(),
  {
    variant: 'default',
    size: 'md',
    disabled: false,
    block: false
  }
)
</script>

<template>
  <button
    :disabled="disabled"
    class="inline-flex items-center justify-center rounded-lg font-semibold transition focus:outline-none focus:ring-2 focus:ring-sea/30 disabled:cursor-not-allowed disabled:opacity-60"
    :class="[
      block ? 'w-full' : '',
      size === 'sm' ? 'h-8 px-3 text-xs' : 'h-9 px-3.5 text-sm',
      variant === 'default' && 'bg-gradient-to-br from-sea to-brand text-white hover:opacity-95',
      variant === 'secondary' && 'bg-slate-100 text-slate-800 hover:bg-slate-200',
      variant === 'danger' && 'bg-rose-600 text-white hover:bg-rose-700',
      variant === 'ghost' && 'bg-transparent text-slate-700 hover:bg-slate-100'
    ]"
  >
    <slot />
  </button>
</template>
