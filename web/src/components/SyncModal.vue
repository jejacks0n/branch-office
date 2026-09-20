<script setup lang="ts">
import { ref } from 'vue'
import { UploadCloud, ArrowUp, ArrowDown, ShieldAlert, X, AlertTriangle } from 'lucide-vue-next'
import ConfirmModal from './ConfirmModal.vue'

const props = defineProps<{
  show: boolean
  branch: string
  upstream: string
  ahead: number
  behind: number
}>()

const emit = defineEmits<{
  (e: 'push', options: { forceWithLease: boolean; setUpstream: boolean }): void
  (e: 'close'): void
}>()

const showForceConfirm = ref(false)
const setUpstream = ref(false)

function handleRegularPush() {
  emit('push', {
    forceWithLease: false,
    setUpstream: !props.upstream || setUpstream.value,
  })
}

function handleForcePushConfirm() {
  showForceConfirm.value = false
  emit('push', {
    forceWithLease: true,
    setUpstream: !props.upstream || setUpstream.value,
  })
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="show"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-fade-in"
      @click.self="emit('close')"
    >
      <div
        class="w-full max-w-md bg-zinc-900 border border-zinc-800 rounded-3xl shadow-2xl p-5 space-y-5"
      >
      <!-- Header -->
      <div class="flex items-center justify-between pb-3 border-b border-zinc-800">
        <div class="flex items-center gap-2.5">
          <div class="p-2 rounded-xl bg-zinc-800 text-sky-400">
            <UploadCloud class="w-5 h-5" />
          </div>
          <div>
            <h3 class="text-base font-semibold text-zinc-100">Push to Remote</h3>
            <p class="text-xs text-zinc-400 font-mono">{{ branch }}</p>
          </div>
        </div>

        <button
          type="button"
          class="p-2 rounded-xl text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors"
          @click="emit('close')"
        >
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Sync Status Overview -->
      <div class="p-4 rounded-2xl bg-zinc-950/60 border border-zinc-800/80 space-y-3">
        <div class="flex items-center justify-between text-xs">
          <span class="text-zinc-400">Upstream:</span>
          <span class="font-mono text-zinc-200">{{ upstream || 'No upstream branch set' }}</span>
        </div>

        <div class="flex items-center gap-3 pt-1">
          <div class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs font-mono font-medium">
            <ArrowUp class="w-3.5 h-3.5" />
            <span>{{ ahead }} ahead</span>
          </div>

          <div class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-zinc-800 border border-zinc-700/60 text-zinc-400 text-xs font-mono font-medium">
            <ArrowDown class="w-3.5 h-3.5" />
            <span>{{ behind }} behind</span>
          </div>
        </div>
      </div>

      <!-- Set upstream toggle if needed -->
      <div v-if="!upstream" class="flex items-center gap-2 text-xs text-zinc-300">
        <input
          type="checkbox"
          id="setupstream"
          v-model="setUpstream"
          class="rounded border-zinc-700 text-emerald-500 bg-zinc-800 w-4 h-4 accent-emerald-500"
        />
        <label for="setupstream" class="cursor-pointer">Set upstream to origin/{{ branch }}</label>
      </div>

      <!-- Action Buttons -->
      <div class="space-y-2.5 pt-2">
        <!-- Regular Push Button -->
        <button
          type="button"
          class="w-full py-3 px-4 rounded-xl text-sm font-semibold bg-emerald-600 hover:bg-emerald-500 active:bg-emerald-700 text-white flex items-center justify-center gap-2 shadow-lg active:scale-[0.99] transition-all"
          @click="handleRegularPush"
        >
          <UploadCloud class="w-4 h-4" />
          <span>Push to Remote</span>
        </button>

        <!-- Force Push Button with Lease -->
        <button
          type="button"
          class="w-full py-2.5 px-4 rounded-xl text-xs font-semibold bg-zinc-800 hover:bg-red-950/40 text-red-400 hover:text-red-300 border border-zinc-700/60 hover:border-red-500/40 flex items-center justify-center gap-2 transition-all active:scale-[0.99]"
          @click="showForceConfirm = true"
        >
          <ShieldAlert class="w-4 h-4" />
          <span>Force Push (--force-with-lease)</span>
        </button>
      </div>
    </div>

    <!-- Force Push Danger Confirmation Dialog -->
    <ConfirmModal
      :show="showForceConfirm"
      title="Confirm Force Push"
      :message="`Are you sure you want to force push to '${branch}'? This uses --force-with-lease for safety, but will rewrite remote branch commits.`"
      confirm-text="Force Push"
      @cancel="showForceConfirm = false"
      @confirm="handleForcePushConfirm"
    />
  </div>
  </Teleport>
</template>
