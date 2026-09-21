<script setup lang="ts">
import { ref, watch, computed, onMounted, onUnmounted } from 'vue'
import type { FileDiff, Hunk } from '../types'
import {
  X,
  Plus,
  Minus,
  RotateCcw,
  ChevronDown,
  ChevronRight,
  ChevronLeft,
  FileCode,
} from 'lucide-vue-next'
import ConfirmModal from './ConfirmModal.vue'

const props = defineProps<{
  show: boolean
  fileDiff: FileDiff | null
  staged: boolean
  untracked?: boolean
  readOnly?: boolean
  hasPrev?: boolean
  hasNext?: boolean
  fileIndex?: number
  totalFiles?: number
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'prev'): void
  (e: 'next'): void
  (e: 'stage-hunk', patch: string): void
  (e: 'unstage-hunk', patch: string): void
  (e: 'discard-hunk', patch: string): void
}>()

const collapsedHunks = ref<Record<number, boolean>>({})
const hunkToDiscard = ref<Hunk | null>(null)
const actionPending = ref(false)

watch(
  () => props.fileDiff,
  () => {
    collapsedHunks.value = {}
  }
)

function toggleHunk(index: number) {
  collapsedHunks.value[index] = !collapsedHunks.value[index]
}

const filePathInfo = computed(() => {
  const full = props.fileDiff?.newPath || props.fileDiff?.oldPath || ''
  const lastSlash = full.lastIndexOf('/')
  if (lastSlash === -1) {
    return { dir: '', file: full, full }
  }
  return {
    dir: full.slice(0, lastSlash + 1),
    file: full.slice(lastSlash + 1),
    full,
  }
})

async function handleStageHunk(hunk: Hunk) {
  if (actionPending.value) return
  actionPending.value = true
  emit('stage-hunk', hunk.patch)
  actionPending.value = false
}

async function handleUnstageHunk(hunk: Hunk) {
  if (actionPending.value) return
  actionPending.value = true
  emit('unstage-hunk', hunk.patch)
  actionPending.value = false
}

function promptDiscardHunk(hunk: Hunk) {
  hunkToDiscard.value = hunk
}

function confirmDiscardHunk() {
  if (hunkToDiscard.value) {
    emit('discard-hunk', hunkToDiscard.value.patch)
    hunkToDiscard.value = null
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (!props.show) return
  if (['INPUT', 'TEXTAREA'].includes((e.target as HTMLElement)?.tagName)) return

  if (e.key === 'ArrowLeft' || e.key === '[') {
    if (props.hasPrev) {
      e.preventDefault()
      emit('prev')
    }
  } else if (e.key === 'ArrowRight' || e.key === ']') {
    if (props.hasNext) {
      e.preventDefault()
      emit('next')
    }
  } else if (e.key === 'Escape') {
    e.preventDefault()
    emit('close')
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="show && fileDiff"
      class="viewport-fixed z-50 flex flex-col bg-zinc-950 text-zinc-100 overflow-hidden animate-fade-in"
    >
    <!-- Sticky Top Header -->
    <header
      class="pt-safe px-4 py-3 bg-zinc-900/95 backdrop-blur border-b border-zinc-800 flex items-center justify-between shrink-0"
    >
      <div class="flex items-center gap-2.5 min-w-0 pr-3">
        <div class="p-2 rounded-xl bg-zinc-800 text-zinc-300 shrink-0">
          <FileCode class="w-4 h-4 text-emerald-400" />
        </div>
        <div class="min-w-0">
          <!-- Top Row: File Name prominently displayed -->
          <h2 class="font-bold text-sm text-zinc-100 truncate font-mono" :title="filePathInfo.full">
            {{ filePathInfo.file }}
          </h2>

          <!-- Bottom Row: [Commit/Staged] badge, directory path, change status, and stats -->
          <div class="flex items-center gap-2 text-xs font-mono mt-0.5 flex-wrap">
            <span
              v-if="readOnly"
              class="text-[10px] uppercase font-bold px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-400 border border-zinc-700/60 shrink-0"
            >
              Commit
            </span>
            <span
              v-else-if="staged"
              class="text-[10px] uppercase font-bold px-1.5 py-0.5 rounded bg-emerald-500/15 text-emerald-400 border border-emerald-500/30 shrink-0"
            >
              Staged
            </span>

            <span
              v-if="filePathInfo.dir"
              class="text-zinc-500 text-[11px] truncate max-w-[140px] sm:max-w-xs shrink"
              :title="filePathInfo.full"
            >
              {{ filePathInfo.dir }}
            </span>

            <span
              v-if="fileDiff.isNew"
              class="text-[10px] uppercase font-bold px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 shrink-0"
            >
              New
            </span>
            <span
              v-else-if="fileDiff.isDeleted"
              class="text-[10px] uppercase font-bold px-1.5 py-0.5 rounded bg-red-500/20 text-red-400 border border-red-500/30 shrink-0"
            >
              Deleted
            </span>
            <span
              v-else-if="fileDiff.isRenamed"
              class="text-[10px] uppercase font-bold px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-400 border border-purple-500/30 shrink-0"
            >
              Renamed
            </span>

            <div class="flex items-center gap-1.5 shrink-0">
              <span class="text-emerald-400 font-medium">+{{ fileDiff.additions }}</span>
              <span class="text-red-400 font-medium">-{{ fileDiff.deletions }}</span>
              <span class="text-zinc-500 text-[11px] font-sans hidden sm:inline">
                ({{ fileDiff.hunks.length }} {{ fileDiff.hunks.length === 1 ? 'hunk' : 'hunks' }})
              </span>
            </div>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <!-- Prev / Next File Navigator -->
        <div
          v-if="totalFiles && totalFiles > 1"
          class="flex items-center bg-zinc-800/80 border border-zinc-700/60 rounded-xl p-0.5"
        >
          <button
            type="button"
            :disabled="!hasPrev"
            class="p-1.5 rounded-lg text-zinc-300 hover:text-zinc-100 hover:bg-zinc-700/60 disabled:opacity-25 disabled:hover:bg-transparent disabled:hover:text-zinc-300 transition-colors"
            title="Previous file (Left Arrow or [)"
            @click="emit('prev')"
          >
            <ChevronLeft class="w-4 h-4" />
          </button>
          <span class="text-[11px] font-mono text-zinc-400 px-1.5 select-none font-medium">
            {{ (fileIndex ?? 0) + 1 }} / {{ totalFiles }}
          </span>
          <button
            type="button"
            :disabled="!hasNext"
            class="p-1.5 rounded-lg text-zinc-300 hover:text-zinc-100 hover:bg-zinc-700/60 disabled:opacity-25 disabled:hover:bg-transparent disabled:hover:text-zinc-300 transition-colors"
            title="Next file (Right Arrow or ])"
            @click="emit('next')"
          >
            <ChevronRight class="w-4 h-4" />
          </button>
        </div>

        <button
          type="button"
          class="p-2 rounded-xl text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800 transition-colors shrink-0"
          title="Close diff (Esc)"
          @click="emit('close')"
        >
          <X class="w-5 h-5" />
        </button>
      </div>
    </header>

    <!-- Main Diff Content -->
    <div class="flex-1 overflow-y-auto p-3 sm:p-4 space-y-4 pb-safe">
      <!-- Binary File Warning -->
      <div
        v-if="fileDiff.isBinary"
        class="p-8 text-center bg-zinc-900/50 rounded-2xl border border-zinc-800 text-zinc-400 space-y-2"
      >
        <p class="text-sm font-medium">Binary file changes</p>
        <p class="text-xs text-zinc-500">Visual diff not available for binary files.</p>
      </div>

      <!-- No Changes / Clean State -->
      <div
        v-else-if="fileDiff.hunks.length === 0"
        class="p-8 text-center bg-zinc-900/50 rounded-2xl border border-zinc-800 text-zinc-500 text-sm"
      >
        No text changes found.
      </div>

      <!-- Hunks -->
      <div
        v-for="hunk in fileDiff.hunks"
        :key="hunk.index"
        class="bg-zinc-900/80 border border-zinc-800/90 rounded-2xl overflow-hidden shadow-sm"
      >
        <!-- Hunk Header & Action Toolbar -->
        <div
          class="flex items-center justify-between px-3.5 py-2.5 bg-zinc-900 border-b border-zinc-800/80 text-xs font-mono"
        >
          <button
            type="button"
            class="flex items-center gap-1.5 text-zinc-400 hover:text-zinc-200 text-left min-w-0 pr-2 truncate"
            @click="toggleHunk(hunk.index)"
          >
            <component
              :is="collapsedHunks[hunk.index] ? ChevronRight : ChevronDown"
              class="w-3.5 h-3.5 shrink-0"
            />
            <span class="text-sky-400 font-semibold truncate">{{ hunk.header }}</span>
          </button>

          <!-- Per-Hunk Actions -->
          <div v-if="!readOnly" class="flex items-center gap-1.5 shrink-0">
            <!-- If unstaged, show Stage and Discard -->
            <template v-if="!staged && !untracked">
              <button
                type="button"
                class="px-2.5 py-1 rounded-lg text-xs font-medium bg-emerald-500/10 text-emerald-400 hover:bg-emerald-500/20 border border-emerald-500/30 flex items-center gap-1 active:scale-95 transition-all"
                @click="handleStageHunk(hunk)"
              >
                <Plus class="w-3.5 h-3.5" />
                <span>Stage</span>
              </button>
              <button
                type="button"
                class="p-1 rounded-lg text-zinc-400 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                title="Discard"
                @click="promptDiscardHunk(hunk)"
              >
                <RotateCcw class="w-3.5 h-3.5" />
              </button>
            </template>

            <!-- If staged, show Unstage -->
            <template v-else-if="staged">
              <button
                type="button"
                class="px-2.5 py-1 rounded-lg text-xs font-medium bg-amber-500/10 text-amber-400 hover:bg-amber-500/20 border border-amber-500/30 flex items-center gap-1 active:scale-95 transition-all"
                @click="handleUnstageHunk(hunk)"
              >
                <Minus class="w-3.5 h-3.5" />
                <span>Unstage</span>
              </button>
            </template>
          </div>
        </div>

        <!-- Hunk Lines -->
        <div v-show="!collapsedHunks[hunk.index]" class="overflow-x-auto text-[12px] font-mono leading-relaxed">
          <table class="w-full border-collapse">
            <tbody>
              <tr
                v-for="(line, lineIdx) in hunk.lines"
                :key="lineIdx"
                :class="[
                  'hover:brightness-110 transition-colors font-mono select-text',
                  line.type === 'addition'
                    ? 'bg-emerald-950/30 text-emerald-200'
                    : line.type === 'deletion'
                    ? 'bg-red-950/30 text-red-200'
                    : line.type === 'meta'
                    ? 'bg-zinc-950 text-zinc-500 italic'
                    : 'text-zinc-300',
                ]"
              >
                <!-- Old Line # -->
                <td
                  class="w-10 px-2 py-0.5 text-right text-[11px] text-zinc-600 select-none border-r border-zinc-800/40"
                >
                  {{ line.oldLine || '' }}
                </td>

                <!-- New Line # -->
                <td
                  class="w-10 px-2 py-0.5 text-right text-[11px] text-zinc-600 select-none border-r border-zinc-800/40"
                >
                  {{ line.newLine || '' }}
                </td>

                <!-- Prefix Marker (+, -, space) -->
                <td
                  :class="[
                    'w-5 px-1.5 py-0.5 text-center select-none font-bold',
                    line.type === 'addition'
                      ? 'text-emerald-400'
                      : line.type === 'deletion'
                      ? 'text-red-400'
                      : 'text-zinc-600',
                  ]"
                >
                  {{ line.type === 'addition' ? '+' : line.type === 'deletion' ? '-' : ' ' }}
                </td>

                <!-- Line Content -->
                <td class="px-2 py-0.5 whitespace-pre overflow-visible">
                  {{ line.content }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- Confirm Discard Changes Modal -->
    <ConfirmModal
      :show="!!hunkToDiscard"
      title="Discard Changes"
      message="Are you sure you want to discard this specific change? This action will permanently revert these lines in your working tree."
      confirm-text="Discard"
      @cancel="hunkToDiscard = null"
      @confirm="confirmDiscardHunk"
    />
  </div>
  </Teleport>
</template>
