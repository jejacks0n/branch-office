<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import {
  Archive,
  ArchiveRestore,
  Plus,
  X,
  Trash2,
  Loader2,
  AlertCircle,
  ChevronDown,
  ChevronUp,
  FileCode,
} from 'lucide-vue-next'
import type { StashItem } from '../types'
import { api } from '../api'
import ConfirmModal from './ConfirmModal.vue'

const props = defineProps<{
  show: boolean
  repoId: string
  repoName: string
  changeCount: number
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'changed'): void
}>()

const stashes = ref<StashItem[]>([])
const loading = ref(false)
const isSubmitting = ref(false)
const modalError = ref<string | null>(null)

// Stash create form
const showNewForm = ref(false)
const stashMessage = ref('')
const includeUntracked = ref(true)

// Diff preview state
const expandedDiffs = ref<Record<number, string>>({})
const loadingDiffIndex = ref<number | null>(null)

// Confirm dialogs
const stashToDrop = ref<StashItem | null>(null)
const confirmClearAll = ref(false)

const canStash = computed(() => props.changeCount > 0)

watch(
  [() => props.show, () => props.repoId],
  async ([show, repoId]) => {
    if (show && repoId) {
      modalError.value = null
      expandedDiffs.value = {}
      showNewForm.value = false
      stashMessage.value = ''
      await loadStashes()
      if (props.changeCount > 0 && (!stashes.value || stashes.value.length === 0)) {
        showNewForm.value = true
      }
    } else {
      stashes.value = []
      showNewForm.value = false
      modalError.value = null
    }
  }
)

async function loadStashes() {
  if (!props.repoId) {
    stashes.value = []
    return
  }
  loading.value = true
  try {
    const res = await api.getStashes(props.repoId)
    stashes.value = Array.isArray(res) ? res : []
  } catch (err: any) {
    stashes.value = []
    modalError.value = err.message || 'Failed to load stashes'
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!props.repoId || isSubmitting.value || !canStash.value) return
  isSubmitting.value = true
  modalError.value = null
  try {
    await api.createStash(props.repoId, stashMessage.value.trim(), includeUntracked.value)
    stashMessage.value = ''
    showNewForm.value = false
    await loadStashes()
    emit('changed')
  } catch (err: any) {
    modalError.value = err.message || 'Failed to save stash'
  } finally {
    isSubmitting.value = false
  }
}

async function handlePop(stash: StashItem) {
  if (!props.repoId || isSubmitting.value) return
  isSubmitting.value = true
  modalError.value = null
  try {
    await api.popStash(props.repoId, stash.index)
    delete expandedDiffs.value[stash.index]
    await loadStashes()
    emit('changed')
  } catch (err: any) {
    modalError.value = err.message || `Failed to pop stash@{${stash.index}}`
  } finally {
    isSubmitting.value = false
  }
}

async function handleApply(stash: StashItem) {
  if (!props.repoId || isSubmitting.value) return
  isSubmitting.value = true
  modalError.value = null
  try {
    await api.applyStash(props.repoId, stash.index)
    emit('changed')
  } catch (err: any) {
    modalError.value = err.message || `Failed to apply stash@{${stash.index}}`
  } finally {
    isSubmitting.value = false
  }
}

function promptDrop(stash: StashItem) {
  stashToDrop.value = stash
}

async function confirmDrop() {
  if (!props.repoId || !stashToDrop.value) return
  const idx = stashToDrop.value.index
  stashToDrop.value = null
  isSubmitting.value = true
  modalError.value = null
  try {
    await api.dropStash(props.repoId, idx)
    delete expandedDiffs.value[idx]
    await loadStashes()
    emit('changed')
  } catch (err: any) {
    modalError.value = err.message || `Failed to drop stash@{${idx}}`
  } finally {
    isSubmitting.value = false
  }
}

async function confirmClear() {
  if (!props.repoId) return
  confirmClearAll.value = false
  isSubmitting.value = true
  modalError.value = null
  try {
    await api.clearStashes(props.repoId)
    expandedDiffs.value = {}
    await loadStashes()
    emit('changed')
  } catch (err: any) {
    modalError.value = err.message || 'Failed to clear stashes'
  } finally {
    isSubmitting.value = false
  }
}

async function toggleDiff(stash: StashItem) {
  if (expandedDiffs.value[stash.index] !== undefined) {
    delete expandedDiffs.value[stash.index]
    return
  }
  loadingDiffIndex.value = stash.index
  try {
    const res = await api.getStashDiff(props.repoId, stash.index)
    expandedDiffs.value[stash.index] = res.diff || 'No text diff available (binary or empty).'
  } catch (err: any) {
    modalError.value = err.message || `Failed to load diff for stash@{${stash.index}}`
  } finally {
    loadingDiffIndex.value = null
  }
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
        class="modal-panel max-w-xl bg-zinc-900 border border-zinc-800 rounded-3xl shadow-2xl flex flex-col overflow-hidden"
      >
        <!-- Header -->
        <div class="px-5 py-4 border-b border-zinc-800/80 flex items-center justify-between shrink-0">
          <div class="flex items-center gap-2.5">
            <div class="p-2 rounded-xl bg-amber-500/10 text-amber-400">
              <Archive class="w-5 h-5" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h2 class="text-base font-semibold text-zinc-100">Git Stashes</h2>
                <span
                  v-if="stashes.length > 0"
                  class="text-[11px] font-mono px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-semibold"
                >
                  {{ stashes.length }}
                </span>
              </div>
              <p class="text-xs text-zinc-400 font-mono">{{ repoName }}</p>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <button
              v-if="stashes.length > 0"
              type="button"
              class="px-2.5 py-1 text-xs font-semibold text-zinc-400 hover:text-red-400 hover:bg-red-500/10 rounded-xl transition-colors border border-transparent hover:border-red-500/20"
              @click="confirmClearAll = true"
            >
              Clear All
            </button>
            <button
              type="button"
              class="p-2 rounded-xl text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors"
              @click="emit('close')"
            >
              <X class="w-5 h-5" />
            </button>
          </div>
        </div>

        <!-- Error Alert -->
        <div
          v-if="modalError"
          class="mx-4 mt-3 p-3 rounded-2xl bg-red-500/10 border border-red-500/20 text-red-400 text-xs flex items-start gap-2 shrink-0"
        >
          <AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
          <span class="font-mono flex-1 whitespace-pre-wrap">{{ modalError }}</span>
          <button
            type="button"
            class="text-red-400 hover:text-red-200 font-bold shrink-0 ml-1"
            @click="modalError = null"
          >
            ✕
          </button>
        </div>

        <!-- Action Bar -->
        <div class="p-3.5 border-b border-zinc-800/80 bg-zinc-950/40 flex items-center justify-between gap-3 shrink-0">
          <div class="flex items-center gap-2 min-w-0">
            <span class="text-xs text-zinc-400">
              Working changes:
              <strong :class="changeCount > 0 ? 'text-amber-400' : 'text-zinc-500'">
                {{ changeCount }} {{ changeCount === 1 ? 'file' : 'files' }}
              </strong>
            </span>
          </div>

          <button
            type="button"
            :class="[
              'px-3 py-1.5 rounded-xl border text-xs font-semibold flex items-center gap-1.5 transition-all active:scale-95 shrink-0',
              showNewForm
                ? 'bg-zinc-800 text-zinc-300 border-zinc-700'
                : canStash
                ? 'bg-amber-600/20 hover:bg-amber-600/30 text-amber-400 border-amber-500/30'
                : 'bg-zinc-800/50 text-zinc-500 border-zinc-800 cursor-not-allowed',
            ]"
            :disabled="!canStash && !showNewForm"
            @click="showNewForm = !showNewForm"
          >
            <Plus v-if="!showNewForm" class="w-3.5 h-3.5" />
            <X v-else class="w-3.5 h-3.5" />
            <span>{{ showNewForm ? 'Cancel' : 'Stash Changes' }}</span>
          </button>
        </div>

        <!-- Stash Creation Form -->
        <div
          v-if="showNewForm"
          class="p-4 border-b border-zinc-800 bg-zinc-950/80 space-y-3 shrink-0 animate-fade-in"
        >
          <h3 class="text-xs font-semibold text-zinc-200 uppercase tracking-wider">Create New Stash</h3>

          <div class="space-y-1">
            <label class="text-xs text-zinc-400">Stash Message (Optional)</label>
            <input
              v-model="stashMessage"
              type="text"
              placeholder="e.g. WIP feature login or quick save"
              class="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-xl text-xs text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-amber-500 font-mono"
              @keyup.enter="handleSave"
            />
          </div>

          <div class="flex items-center justify-between pt-1">
            <label class="flex items-center gap-2 cursor-pointer select-none text-xs text-zinc-300">
              <input
                v-model="includeUntracked"
                type="checkbox"
                class="rounded border-zinc-700 bg-zinc-900 text-amber-500 focus:ring-0 focus:ring-offset-0"
              />
              <span>Include untracked files (<code class="text-zinc-400 font-mono">-u</code>)</span>
            </label>

            <button
              type="button"
              :disabled="!canStash || isSubmitting"
              class="px-4 py-2 bg-amber-600 hover:bg-amber-500 disabled:opacity-40 text-white rounded-xl text-xs font-semibold flex items-center gap-1.5 transition-all shadow-md active:scale-95"
              @click="handleSave"
            >
              <Loader2 v-if="isSubmitting" class="w-3.5 h-3.5 animate-spin" />
              <Archive v-else class="w-3.5 h-3.5" />
              <span>Save Stash</span>
            </button>
          </div>
        </div>

        <!-- Stashes List -->
        <div class="flex-1 overflow-y-auto p-4 space-y-2 min-h-[200px]">
          <div v-if="loading" class="py-12 text-center text-zinc-500 space-y-2">
            <Loader2 class="w-6 h-6 animate-spin mx-auto text-amber-400" />
            <p class="text-xs">Loading stashes...</p>
          </div>

          <div v-else-if="stashes.length === 0" class="py-12 text-center text-zinc-500 space-y-2">
            <Archive class="w-8 h-8 mx-auto text-zinc-700" />
            <p class="text-xs font-medium text-zinc-400">No stashed changes</p>
            <p class="text-[11px] text-zinc-600 max-w-xs mx-auto">
              Stash uncommitted changes to switch branches cleanly, then pop them back anytime.
            </p>
          </div>

          <div
            v-for="stash in stashes"
            :key="stash.ref"
            class="rounded-2xl border border-zinc-800/80 bg-zinc-900/60 overflow-hidden transition-all group hover:border-zinc-700"
          >
            <!-- Stash Header Row -->
            <div class="p-3 flex items-center justify-between gap-3 min-h-[66px]">
              <!-- Stash Info -->
              <div class="min-w-0 flex-1">
                <div class="h-5 flex items-center gap-2">
                  <Archive class="w-4 h-4 shrink-0 text-amber-400" />
                  <span class="font-mono text-xs font-semibold text-zinc-200">
                    {{ stash.ref }}
                  </span>

                  <span
                    v-if="stash.branch"
                    class="h-5 inline-flex items-center text-[10px] font-mono leading-none px-1.5 rounded bg-zinc-800 text-zinc-400 border border-zinc-700/60 shrink-0 truncate max-w-[120px]"
                    :title="stash.branch"
                  >
                    {{ stash.branch }}
                  </span>

                  <span v-if="stash.hash" class="text-[11px] font-mono text-zinc-500 shrink-0">
                    {{ stash.hash }}
                  </span>

                  <span v-if="stash.date" class="text-[11px] text-zinc-500 truncate">
                    {{ stash.date }}
                  </span>
                </div>

                <!-- Stash Message -->
                <div class="mt-1 h-4 flex items-center gap-2 text-xs font-mono text-zinc-300">
                  <span class="truncate">{{ stash.message }}</span>
                </div>
              </div>

              <!-- Actions: Pop, Apply, Diff, Drop -->
              <div class="shrink-0 flex items-center gap-1.5">
                <!-- Diff Toggle -->
                <button
                  type="button"
                  class="h-7 px-2 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-zinc-400 hover:text-zinc-200 border border-zinc-700/80 text-xs font-medium flex items-center gap-1 transition-all"
                  :title="expandedDiffs[stash.index] !== undefined ? 'Hide diff' : 'Preview diff'"
                  @click="toggleDiff(stash)"
                >
                  <Loader2 v-if="loadingDiffIndex === stash.index" class="w-3.5 h-3.5 animate-spin text-amber-400" />
                  <template v-else>
                    <FileCode class="w-3.5 h-3.5" />
                    <ChevronUp v-if="expandedDiffs[stash.index] !== undefined" class="w-3 h-3" />
                    <ChevronDown v-else class="w-3 h-3" />
                  </template>
                </button>

                <!-- Apply Button -->
                <button
                  type="button"
                  :disabled="isSubmitting"
                  class="h-7 px-2.5 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-zinc-200 border border-zinc-700/80 text-xs font-semibold flex items-center gap-1 transition-all active:scale-95"
                  title="Apply changes (keep in stash)"
                  @click="handleApply(stash)"
                >
                  <span>Apply</span>
                </button>

                <!-- Pop Button -->
                <button
                  type="button"
                  :disabled="isSubmitting"
                  class="h-7 px-3 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold flex items-center gap-1.5 transition-all shadow-sm active:scale-95"
                  title="Apply changes and drop from stash"
                  @click="handlePop(stash)"
                >
                  <ArchiveRestore class="w-3.5 h-3.5" />
                  <span>Pop</span>
                </button>

                <!-- Drop Button -->
                <button
                  type="button"
                  :disabled="isSubmitting"
                  class="h-7 w-7 rounded-xl text-zinc-500 hover:text-red-400 hover:bg-red-500/10 flex items-center justify-center transition-colors"
                  title="Drop stash"
                  @click="promptDrop(stash)"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <!-- Expanded Diff Section -->
            <div
              v-if="expandedDiffs[stash.index] !== undefined"
              class="border-t border-zinc-800/80 bg-zinc-950/70 p-3 text-[11px] font-mono max-h-60 overflow-y-auto leading-relaxed"
            >
              <div class="flex items-center justify-between pb-2 mb-2 border-b border-zinc-800/60 text-zinc-400 text-xs">
                <span>Diff Preview ({{ stash.ref }})</span>
                <button
                  type="button"
                  class="text-zinc-500 hover:text-zinc-300"
                  @click="toggleDiff(stash)"
                >
                  <X class="w-3.5 h-3.5" />
                </button>
              </div>
              <pre class="whitespace-pre-wrap break-words text-zinc-300"><template v-for="(line, lIdx) in expandedDiffs[stash.index].split('\n')" :key="lIdx"><span
                  :class="[
                    line.startsWith('+') && !line.startsWith('+++')
                      ? 'text-emerald-400'
                      : line.startsWith('-') && !line.startsWith('---')
                      ? 'text-red-400'
                      : line.startsWith('@@')
                      ? 'text-purple-400'
                      : line.startsWith('diff ')
                      ? 'text-amber-400 font-bold'
                      : 'text-zinc-400'
                  ]"
                >{{ line }}
</span></template></pre>
            </div>
          </div>
        </div>
      </div>

      <!-- Confirm Drop Stash Modal -->
      <ConfirmModal
        :show="!!stashToDrop"
        title="Drop Stash"
        :message="`Are you sure you want to drop '${stashToDrop?.ref}' (${stashToDrop?.message})? This action cannot be undone.`"
        confirm-text="Drop Stash"
        @cancel="stashToDrop = null"
        @confirm="confirmDrop"
      />

      <!-- Confirm Clear All Stashes Modal -->
      <ConfirmModal
        :show="confirmClearAll"
        title="Clear All Stashes"
        :message="`Are you sure you want to delete all ${stashes.length} stashes in this repository? This action cannot be undone.`"
        confirm-text="Clear All Stashes"
        @cancel="confirmClearAll = false"
        @confirm="confirmClear"
      />
    </div>
  </Teleport>
</template>
