<script setup lang="ts">
import { ref, watch } from 'vue'
import type { Worktree } from '../types'
import { GitFork, Plus, Trash2, X, GitBranch, ArrowRight } from 'lucide-vue-next'
import ConfirmModal from './ConfirmModal.vue'

const props = defineProps<{
  show: boolean
  worktrees: Worktree[]
  currentPath: string
  repoName: string
}>()

const emit = defineEmits<{
  (e: 'switch', path: string): void
  (e: 'add', data: { path: string; branch: string; createBranch: boolean }): void
  (e: 'remove', path: string): void
  (e: 'close'): void
}>()

const showNewForm = ref(false)
const targetPath = ref('')
const branch = ref('')
const createBranch = ref(true)
const isSubmitting = ref(false)
const wtToDelete = ref<Worktree | null>(null)

watch(
  () => props.show,
  (newVal) => {
    if (newVal) {
      showNewForm.value = false
      targetPath.value = ''
      branch.value = ''
      createBranch.value = true
    }
  }
)

function onBranchInput() {
  if (!targetPath.value || targetPath.value.startsWith('../')) {
    const slug = branch.value.trim().replace(/[^a-zA-Z0-9-_]/g, '-')
    if (slug) {
      targetPath.value = `../${props.repoName}-${slug}`
    } else {
      targetPath.value = ''
    }
  }
}

async function handleAdd() {
  if (!targetPath.value.trim() || !branch.value.trim() || isSubmitting.value) return
  isSubmitting.value = true
  try {
    emit('add', {
      path: targetPath.value.trim(),
      branch: branch.value.trim(),
      createBranch: createBranch.value,
    })
    showNewForm.value = false
    targetPath.value = ''
    branch.value = ''
  } finally {
    isSubmitting.value = false
  }
}

function promptDelete(wt: Worktree) {
  wtToDelete.value = wt
}

function confirmDelete() {
  if (wtToDelete.value) {
    emit('remove', wtToDelete.value.path)
    wtToDelete.value = null
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
        class="modal-panel max-w-lg bg-zinc-900 border border-zinc-800 rounded-3xl shadow-2xl flex flex-col overflow-hidden"
      >
      <!-- Header -->
      <div class="px-5 py-4 border-b border-zinc-800/80 flex items-center justify-between shrink-0">
        <div class="flex items-center gap-2.5">
          <div class="p-2 rounded-xl bg-zinc-800 text-teal-400">
            <GitFork class="w-5 h-5" />
          </div>
          <div>
            <h2 class="text-base font-semibold text-zinc-100">Git Worktrees</h2>
            <p class="text-xs text-zinc-400 font-mono">{{ repoName }}</p>
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

      <!-- Action: Toggle New Worktree Form -->
      <div class="p-3.5 border-b border-zinc-800/80 bg-zinc-950/40 flex items-center justify-between">
        <span class="text-xs font-semibold text-zinc-300">
          {{ worktrees.length }} {{ worktrees.length === 1 ? 'Worktree' : 'Worktrees' }} Available
        </span>
        <button
          type="button"
          class="px-3 py-1.5 rounded-xl bg-teal-600/20 hover:bg-teal-600/30 text-teal-400 border border-teal-500/30 text-xs font-semibold flex items-center gap-1.5 transition-all active:scale-95"
          @click="showNewForm = !showNewForm"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>{{ showNewForm ? 'Cancel' : 'New Worktree' }}</span>
        </button>
      </div>

      <!-- New Worktree Form -->
      <div v-if="showNewForm" class="p-4 border-b border-zinc-800 bg-zinc-950/80 space-y-3 animate-fade-in">
        <h3 class="text-xs font-semibold text-zinc-200 uppercase tracking-wider">Create Linked Worktree</h3>

        <div class="space-y-1">
          <label class="text-xs text-zinc-400">Branch Name</label>
          <input
            v-model="branch"
            type="text"
            placeholder="e.g. feature-login"
            class="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-xl text-xs text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-teal-500 font-mono"
            @input="onBranchInput"
          />
        </div>

        <div class="space-y-1">
          <label class="text-xs text-zinc-400">Directory Path</label>
          <input
            v-model="targetPath"
            type="text"
            placeholder="e.g. ../repo-feature-login"
            class="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-xl text-xs text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-teal-500 font-mono"
          />
        </div>

        <div class="flex items-center gap-2 pt-1">
          <input
            type="checkbox"
            id="createBranchToggle"
            v-model="createBranch"
            class="rounded border-zinc-700 text-teal-500 bg-zinc-800 w-4 h-4 accent-teal-500"
          />
          <label for="createBranchToggle" class="text-xs text-zinc-300 cursor-pointer">
            Create new branch (-b)
          </label>
        </div>

        <div class="pt-2 flex justify-end">
          <button
            type="button"
            :disabled="!targetPath.trim() || !branch.trim() || isSubmitting"
            class="px-4 py-2 bg-teal-600 hover:bg-teal-500 disabled:opacity-40 text-white rounded-xl text-xs font-semibold flex items-center gap-1.5 transition-all shadow-md active:scale-95"
            @click="handleAdd"
          >
            <Plus class="w-4 h-4" />
            <span>Create & Register Worktree</span>
          </button>
        </div>
      </div>

      <!-- Worktrees List -->
      <div class="flex-1 overflow-y-auto p-4 space-y-2">
        <div
          v-for="wt in worktrees"
          :key="wt.path"
          :class="[
            'p-3.5 rounded-2xl border transition-all flex items-center justify-between gap-3',
            wt.path === currentPath
              ? 'bg-teal-950/20 border-teal-500/40'
              : 'bg-zinc-900/60 border-zinc-800/80 hover:bg-zinc-800/60',
          ]"
        >
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <GitBranch class="w-4 h-4 text-teal-400 shrink-0" />
              <span class="font-mono text-xs font-semibold text-zinc-100 truncate">
                {{ wt.branch }}
              </span>
              <span
                v-if="wt.isMain"
                class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-400 border border-zinc-700 shrink-0"
              >
                main
              </span>
              <span
                v-if="wt.isLocked"
                class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20 shrink-0"
              >
                locked
              </span>
            </div>
            <div class="text-[11px] text-zinc-500 font-mono truncate mt-1">
              {{ wt.path }}
            </div>
          </div>

          <!-- Actions (only for non-current worktrees) -->
          <div v-if="wt.path !== currentPath" class="flex items-center gap-1.5 shrink-0">
            <button
              type="button"
              class="px-3 py-1.5 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-zinc-200 text-xs font-semibold flex items-center gap-1 transition-all active:scale-95"
              @click="emit('switch', wt.path)"
            >
              <span>Switch</span>
              <ArrowRight class="w-3 h-3" />
            </button>
            <button
              v-if="!wt.isMain"
              type="button"
              class="p-2 rounded-lg text-zinc-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
              title="Remove worktree"
              @click="promptDelete(wt)"
            >
              <Trash2 class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Confirm Remove Worktree Modal -->
    <ConfirmModal
      :show="!!wtToDelete"
      title="Remove Worktree"
      :message="`Are you sure you want to remove the worktree at '${wtToDelete?.path}' (branch: ${wtToDelete?.branch})? This will delete the worktree from Git.`"
      confirm-text="Remove Worktree"
      @cancel="wtToDelete = null"
      @confirm="confirmDelete"
    />
  </div>
  </Teleport>
</template>
