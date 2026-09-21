<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  AlertTriangle,
  ArrowDown,
  ArrowUp,
  DownloadCloud,
  RefreshCw,
  ShieldAlert,
  UploadCloud,
  X,
} from 'lucide-vue-next'
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
  (e: 'pull', options: { rebase: boolean; setUpstream: boolean }): void
  (e: 'fetch'): void
  (e: 'close'): void
}>()

const showForceConfirm = ref(false)
const setUpstream = ref(false)
const rebase = ref(false)

const isDetached = computed(() => !props.branch || props.branch === 'HEAD' || props.branch.includes('(detached)'))

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

function handlePull() {
  emit('pull', {
    rebase: rebase.value,
    setUpstream: !props.upstream || setUpstream.value,
  })
}

function handleFetch() {
  emit('fetch')
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="show"
      class="viewport-fixed modal-overlay z-50 bg-black/75 backdrop-blur-sm animate-fade-in"
      @click.self="emit('close')"
    >
      <div
        class="modal-panel max-w-md bg-zinc-900 border border-zinc-800 rounded-3xl shadow-2xl p-5 space-y-5 overflow-y-auto"
      >
        <!-- Header -->
        <div class="flex items-center justify-between pb-3 border-b border-zinc-800">
          <div class="flex items-center gap-2.5 min-w-0">
            <div class="p-2 rounded-xl bg-zinc-800 text-sky-400 shrink-0">
              <RefreshCw class="w-5 h-5" />
            </div>
            <div class="min-w-0">
              <h3 class="text-base font-semibold text-zinc-100">Remote Sync</h3>
              <p class="text-xs text-zinc-400 font-mono truncate">{{ branch }}</p>
            </div>
          </div>

          <button
            type="button"
            class="p-2 rounded-xl text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors shrink-0"
            @click="emit('close')"
          >
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Sync Status Overview -->
        <div class="p-4 rounded-2xl bg-zinc-950/60 border border-zinc-800/80 space-y-3">
          <div class="flex items-center justify-between text-xs">
            <span class="text-zinc-400">Upstream:</span>
            <span class="font-mono text-zinc-200 truncate ml-2 text-right">
              {{ upstream || 'No upstream branch set' }}
            </span>
          </div>

          <div class="flex items-center justify-between pt-1 gap-2 flex-wrap sm:flex-nowrap">
            <div class="flex items-center gap-2">
              <div
                :class="[
                  'flex items-center gap-1.5 px-3 py-1.5 rounded-xl border text-xs font-mono font-medium',
                  ahead > 0
                    ? 'bg-sky-500/10 border-sky-500/20 text-sky-400'
                    : 'bg-zinc-800 border-zinc-700/60 text-zinc-400',
                ]"
              >
                <ArrowUp class="w-3.5 h-3.5" />
                <span>{{ ahead }} ahead</span>
              </div>

              <div
                :class="[
                  'flex items-center gap-1.5 px-3 py-1.5 rounded-xl border text-xs font-mono font-medium',
                  behind > 0
                    ? 'bg-amber-500/10 border-amber-500/30 text-amber-400'
                    : 'bg-zinc-800 border-zinc-700/60 text-zinc-400',
                ]"
              >
                <ArrowDown class="w-3.5 h-3.5" />
                <span>{{ behind }} behind</span>
              </div>
            </div>

            <!-- Fetch latest button -->
            <button
              type="button"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-zinc-800 hover:bg-zinc-700 border border-zinc-700/60 text-zinc-300 hover:text-white text-xs font-medium transition-all active:scale-95 shrink-0"
              title="Fetch latest remote branches and commits"
              @click="handleFetch"
            >
              <RefreshCw class="w-3.5 h-3.5" />
              <span>Fetch</span>
            </button>
          </div>
        </div>

        <!-- Pull Section -->
        <div class="space-y-2">
          <button
            type="button"
            :class="[
              'w-full py-3 px-4 rounded-xl text-sm font-semibold flex items-center justify-center gap-2 shadow-lg active:scale-[0.99] transition-all',
              behind > 0
                ? 'bg-emerald-600 hover:bg-emerald-500 active:bg-emerald-700 text-white'
                : 'bg-zinc-800/90 hover:bg-zinc-700 text-zinc-200 border border-zinc-700/60',
            ]"
            @click="handlePull"
          >
            <DownloadCloud class="w-4 h-4" />
            <span>Pull from Remote</span>
          </button>

          <div class="flex items-center justify-between text-xs text-zinc-400 px-1">
            <label class="flex items-center gap-2 cursor-pointer hover:text-zinc-200 select-none">
              <input
                type="checkbox"
                v-model="rebase"
                class="rounded border-zinc-700 text-emerald-500 bg-zinc-800 w-4 h-4 accent-emerald-500"
              />
              <span>Rebase local commits (--rebase)</span>
            </label>
          </div>
        </div>

        <!-- Push Section -->
        <div class="border-t border-zinc-800/80 pt-3 space-y-2.5">
          <!-- Detached HEAD Warning -->
          <div
            v-if="isDetached"
            class="p-3 rounded-2xl bg-amber-500/10 border border-amber-500/20 text-amber-300 text-xs flex items-center gap-2"
          >
            <AlertTriangle class="w-4 h-4 shrink-0" />
            <span>HEAD is currently detached (e.g. rebase in progress). Push is disabled until a branch is checked out.</span>
          </div>

          <!-- Set upstream toggle if needed -->
          <div v-if="!upstream && !isDetached" class="flex items-center gap-2 text-xs text-zinc-300 px-1">
            <input
              type="checkbox"
              id="setupstream"
              v-model="setUpstream"
              class="rounded border-zinc-700 text-emerald-500 bg-zinc-800 w-4 h-4 accent-emerald-500"
            />
            <label for="setupstream" class="cursor-pointer select-none">
              Set upstream to origin/{{ branch }}
            </label>
          </div>

          <!-- Regular Push Button -->
          <button
            type="button"
            :disabled="isDetached"
            :class="[
              'w-full py-3 px-4 rounded-xl text-sm font-semibold flex items-center justify-center gap-2 shadow-lg active:scale-[0.99] transition-all disabled:opacity-40 disabled:cursor-not-allowed',
              ahead > 0 && behind === 0 && !isDetached
                ? 'bg-emerald-600 hover:bg-emerald-500 active:bg-emerald-700 text-white'
                : 'bg-zinc-800/90 hover:bg-zinc-700 text-zinc-200 border border-zinc-700/60',
            ]"
            @click="handleRegularPush"
          >
            <UploadCloud class="w-4 h-4" />
            <span>Push to Remote</span>
          </button>

          <!-- Force Push Button with Lease -->
          <button
            type="button"
            :disabled="isDetached"
            class="w-full py-2.5 px-4 rounded-xl text-xs font-semibold bg-zinc-800/80 hover:bg-red-950/40 text-red-400 hover:text-red-300 border border-zinc-700/60 hover:border-red-500/40 flex items-center justify-center gap-2 transition-all active:scale-[0.99] disabled:opacity-40 disabled:cursor-not-allowed"
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
