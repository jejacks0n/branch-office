<script setup lang="ts">
import { ref, computed } from 'vue'
import type { RepoStatus, FileStatus } from '../types'
import {
  FileCode,
  Plus,
  Minus,
  RotateCcw,
  CheckCheck,
  Eye,
  Trash2,
} from 'lucide-vue-next'
import ConfirmModal from './ConfirmModal.vue'

const props = defineProps<{
  status: RepoStatus
}>()

const emit = defineEmits<{
  (e: 'stage-files', files: string[]): void
  (e: 'unstage-files', files: string[]): void
  (e: 'stage-all'): void
  (e: 'unstage-all'): void
  (e: 'discard-files', files: string[]): void
  (e: 'discard-all'): void
  (e: 'view-diff', file: FileStatus, staged: boolean): void
}>()

const fileToDiscard = ref<FileStatus | null>(null)
const discardAllPrompt = ref(false)

const stagedFiles = computed(() => props.status.files.filter((f) => f.isStaged))
const unstagedFiles = computed(() => props.status.files.filter((f) => f.isUnstaged && !f.isUntracked))
const untrackedFiles = computed(() => props.status.files.filter((f) => f.isUntracked))

function formatPath(filePath: string) {
  const lastSlash = filePath.lastIndexOf('/')
  if (lastSlash === -1) {
    return { dir: '', file: filePath }
  }
  return {
    dir: filePath.slice(0, lastSlash + 1),
    file: filePath.slice(lastSlash + 1),
  }
}

function getStatusBadge(status: string) {
  switch (status) {
    case 'modified':
      return { text: 'M', class: 'bg-amber-500/10 text-amber-400 border-amber-500/20' }
    case 'added':
      return { text: 'A', class: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' }
    case 'deleted':
      return { text: 'D', class: 'bg-red-500/10 text-red-400 border-red-500/20' }
    case 'renamed':
      return { text: 'R', class: 'bg-purple-500/10 text-purple-400 border-purple-500/20' }
    case 'untracked':
      return { text: '?', class: 'bg-zinc-500/10 text-zinc-400 border-zinc-500/20' }
    default:
      return { text: '•', class: 'bg-zinc-500/10 text-zinc-400 border-zinc-500/20' }
  }
}

function promptDiscard(file: FileStatus) {
  fileToDiscard.value = file
}

function confirmDiscard() {
  if (fileToDiscard.value) {
    emit('discard-files', [fileToDiscard.value.path])
    fileToDiscard.value = null
  }
}

function confirmDiscardAll() {
  emit('discard-all')
  discardAllPrompt.value = false
}
</script>

<template>
  <div class="space-y-6">
    <!-- Clean Tree Notice -->
    <div
      v-if="status.files.length === 0"
      class="py-16 text-center space-y-3 bg-zinc-900/30 rounded-3xl border border-zinc-800/60 p-6"
    >
      <!-- Decorative: the text below carries the meaning. -->
      <img src="/favicon.svg" alt="" width="67" height="80" class="h-20 w-auto mx-auto" />
      <div class="space-y-1">
        <h3 class="font-semibold text-zinc-200">Working tree clean</h3>
        <p class="text-xs text-zinc-500 max-w-[16rem] mx-auto">
          No uncommitted changes in this repository. Your office plant is thriving.
        </p>
      </div>
    </div>

    <!-- 1. Staged Changes Section -->
    <section v-if="stagedFiles.length > 0" class="space-y-2.5">
      <div class="flex items-center justify-between px-1">
        <div class="flex items-center gap-2">
          <span class="text-xs font-bold uppercase tracking-wider text-emerald-400">Staged</span>
          <span class="text-xs px-2 py-0.5 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 font-mono font-medium">
            {{ stagedFiles.length }}
          </span>
        </div>

        <button
          type="button"
          class="text-xs text-zinc-400 hover:text-zinc-200 hover:underline transition-colors font-medium px-2 py-1"
          @click="emit('unstage-all')"
        >
          Unstage All
        </button>
      </div>

      <div class="space-y-1.5">
        <div
          v-for="file in stagedFiles"
          :key="file.path"
          class="group flex items-center justify-between p-3 rounded-2xl bg-zinc-900/70 hover:bg-zinc-800/80 border border-zinc-800/80 transition-all cursor-pointer active:scale-[0.99]"
          @click="emit('view-diff', file, true)"
        >
          <div class="flex items-center gap-2.5 min-w-0 pr-2">
            <!-- Badge -->
            <span
              :class="[
                'w-5 h-5 rounded-md text-[11px] font-bold font-mono flex items-center justify-center border shrink-0',
                getStatusBadge(file.stagedStatus).class,
              ]"
            >
              {{ getStatusBadge(file.stagedStatus).text }}
            </span>

            <!-- Path -->
            <div class="min-w-0 text-xs font-mono flex items-baseline gap-1.5" :title="file.path">
              <span class="text-zinc-200 font-semibold shrink-0">{{ formatPath(file.path).file }}</span>
              <span v-if="formatPath(file.path).dir" class="text-zinc-500 text-[11px] truncate">{{ formatPath(file.path).dir }}</span>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex items-center gap-1 shrink-0" @click.stop>
            <button
              type="button"
              class="p-2 rounded-xl text-zinc-400 hover:text-amber-400 hover:bg-amber-500/10 transition-colors"
              title="Unstage file"
              @click="emit('unstage-files', [file.path])"
            >
              <Minus class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- 2. Unstaged Changes Section -->
    <section v-if="unstagedFiles.length > 0" class="space-y-2.5">
      <div class="flex items-center justify-between px-1">
        <div class="flex items-center gap-2">
          <span class="text-xs font-bold uppercase tracking-wider text-amber-400">Modified</span>
          <span class="text-xs px-2 py-0.5 rounded-full bg-amber-500/10 border border-amber-500/20 text-amber-400 font-mono font-medium">
            {{ unstagedFiles.length }}
          </span>
        </div>

        <div class="flex items-center gap-2">
          <button
            type="button"
            class="text-xs text-red-400 hover:text-red-300 font-medium px-2 py-1 transition-colors"
            @click="discardAllPrompt = true"
          >
            Discard All
          </button>
          <button
            type="button"
            class="text-xs text-emerald-400 hover:text-emerald-300 font-medium px-2 py-1 transition-colors"
            @click="emit('stage-all')"
          >
            Stage All
          </button>
        </div>
      </div>

      <div class="space-y-1.5">
        <div
          v-for="file in unstagedFiles"
          :key="file.path"
          class="group flex items-center justify-between p-3 rounded-2xl bg-zinc-900/70 hover:bg-zinc-800/80 border border-zinc-800/80 transition-all cursor-pointer active:scale-[0.99]"
          @click="emit('view-diff', file, false)"
        >
          <div class="flex items-center gap-2.5 min-w-0 pr-2">
            <span
              :class="[
                'w-5 h-5 rounded-md text-[11px] font-bold font-mono flex items-center justify-center border shrink-0',
                getStatusBadge(file.unstagedStatus).class,
              ]"
            >
              {{ getStatusBadge(file.unstagedStatus).text }}
            </span>

            <div class="min-w-0 text-xs font-mono flex items-baseline gap-1.5" :title="file.path">
              <span class="text-zinc-200 font-semibold shrink-0">{{ formatPath(file.path).file }}</span>
              <span v-if="formatPath(file.path).dir" class="text-zinc-500 text-[11px] truncate">{{ formatPath(file.path).dir }}</span>
            </div>
          </div>

          <div class="flex items-center gap-1 shrink-0" @click.stop>
            <button
              type="button"
              class="p-2 rounded-xl text-zinc-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
              title="Discard changes"
              @click="promptDiscard(file)"
            >
              <RotateCcw class="w-4 h-4" />
            </button>

            <button
              type="button"
              class="p-2 rounded-xl text-zinc-400 hover:text-emerald-400 hover:bg-emerald-500/10 transition-colors"
              title="Stage file"
              @click="emit('stage-files', [file.path])"
            >
              <Plus class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- 3. Untracked Files Section -->
    <section v-if="untrackedFiles.length > 0" class="space-y-2.5">
      <div class="flex items-center justify-between px-1">
        <div class="flex items-center gap-2">
          <span class="text-xs font-bold uppercase tracking-wider text-zinc-400">Untracked</span>
          <span class="text-xs px-2 py-0.5 rounded-full bg-zinc-800 border border-zinc-700/60 text-zinc-400 font-mono font-medium">
            {{ untrackedFiles.length }}
          </span>
        </div>

        <button
          type="button"
          class="text-xs text-emerald-400 hover:text-emerald-300 font-medium px-2 py-1 transition-colors"
          @click="emit('stage-files', untrackedFiles.map(f => f.path))"
        >
          Stage All
        </button>
      </div>

      <div class="space-y-1.5">
        <div
          v-for="file in untrackedFiles"
          :key="file.path"
          class="group flex items-center justify-between p-3 rounded-2xl bg-zinc-900/70 hover:bg-zinc-800/80 border border-zinc-800/80 transition-all cursor-pointer active:scale-[0.99]"
          @click="emit('view-diff', file, false)"
        >
          <div class="flex items-center gap-2.5 min-w-0 pr-2">
            <span
              :class="[
                'w-5 h-5 rounded-md text-[11px] font-bold font-mono flex items-center justify-center border shrink-0',
                getStatusBadge(file.unstagedStatus).class,
              ]"
            >
              ?
            </span>

            <div class="min-w-0 text-xs font-mono flex items-baseline gap-1.5" :title="file.path">
              <span class="text-zinc-200 font-semibold shrink-0">{{ formatPath(file.path).file }}</span>
              <span v-if="formatPath(file.path).dir" class="text-zinc-500 text-[11px] truncate">{{ formatPath(file.path).dir }}</span>
            </div>
          </div>

          <div class="flex items-center gap-1 shrink-0" @click.stop>
            <button
              type="button"
              class="p-2 rounded-xl text-zinc-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
              title="Delete untracked file"
              @click="promptDiscard(file)"
            >
              <Trash2 class="w-4 h-4" />
            </button>

            <button
              type="button"
              class="p-2 rounded-xl text-zinc-400 hover:text-emerald-400 hover:bg-emerald-500/10 transition-colors"
              title="Stage untracked file"
              @click="emit('stage-files', [file.path])"
            >
              <Plus class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- Discard File Modal -->
    <ConfirmModal
      :show="!!fileToDiscard"
      title="Discard Changes"
      :message="`Are you sure you want to revert changes in '${fileToDiscard?.path}'? This action cannot be undone.`"
      confirm-text="Discard"
      @cancel="fileToDiscard = null"
      @confirm="confirmDiscard"
    />

    <!-- Discard All Modal -->
    <ConfirmModal
      :show="discardAllPrompt"
      title="Discard All Changes"
      message="Are you sure you want to discard ALL unstaged modifications in this repository? Your working tree changes will be permanently lost."
      confirm-text="Discard All"
      @cancel="discardAllPrompt = false"
      @confirm="confirmDiscardAll"
    />
  </div>
</template>
