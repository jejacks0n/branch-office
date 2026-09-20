<script setup lang="ts">
import { AlertTriangle } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    show: boolean
    title: string
    message: string
    confirmText?: string
    cancelText?: string
    isDanger?: boolean
  }>(),
  {
    confirmText: 'Confirm',
    cancelText: 'Cancel',
    isDanger: true,
  }
)

const emit = defineEmits<{
  (e: 'confirm'): void
  (e: 'cancel'): void
}>()
</script>

<template>
  <Teleport to="body">
    <div
      v-if="show"
      class="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-fade-in"
      @click.self="emit('cancel')"
    >
      <div
        class="w-full max-w-sm bg-zinc-900 border border-zinc-800 rounded-2xl shadow-2xl p-5 space-y-4 transform transition-all"
      >
        <div class="flex items-start gap-3">
          <div
            v-if="isDanger"
            class="p-2.5 rounded-xl bg-red-500/10 text-red-400 border border-red-500/20 shrink-0"
          >
            <AlertTriangle class="w-5 h-5" />
          </div>
          <div class="space-y-1">
            <h3 class="text-base font-semibold text-zinc-100">{{ title }}</h3>
            <p class="text-sm text-zinc-400 leading-relaxed">{{ message }}</p>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2.5 pt-2">
          <button
            type="button"
            class="px-4 py-2 text-sm font-medium text-zinc-300 bg-zinc-800 hover:bg-zinc-700 active:bg-zinc-700 rounded-xl transition-colors"
            @click="emit('cancel')"
          >
            {{ cancelText }}
          </button>
          <button
            type="button"
            :class="[
              'px-4 py-2 text-sm font-semibold rounded-xl transition-all shadow-sm active:scale-95',
              isDanger
                ? 'bg-red-600 hover:bg-red-500 text-white'
                : 'bg-emerald-600 hover:bg-emerald-500 text-white',
            ]"
            @click="emit('confirm')"
          >
            {{ confirmText }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
