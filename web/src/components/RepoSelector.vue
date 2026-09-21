<script setup lang="ts">
import { ref } from 'vue'
import type { Repo } from '../types'
import { FolderGit2, Plus, Trash2, X, GitBranch, AlertCircle } from 'lucide-vue-next'
import ConfirmModal from './ConfirmModal.vue'

const props = defineProps<{
  show: boolean
  repos: Repo[]
  activeRepoId?: string
}>()

const emit = defineEmits<{
  (e: 'select', repo: Repo): void
  (e: 'add', path: string): void
  (e: 'remove', repoId: string): void
  (e: 'close'): void
}>()

const newPath = ref('')
const repoToDelete = ref<Repo | null>(null)
const isAdding = ref(false)

async function handleAdd() {
  const path = newPath.value.trim()
  if (!path) return
  isAdding.value = true
  emit('add', path)
  newPath.value = ''
  isAdding.value = false
}

function promptDelete(repo: Repo) {
  repoToDelete.value = repo
}

function confirmDelete() {
  if (repoToDelete.value) {
    emit('remove', repoToDelete.value.id)
    repoToDelete.value = null
  }
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
        class="w-full max-w-lg bg-zinc-900 border border-zinc-800 rounded-3xl shadow-2xl flex flex-col max-h-[85vh] overflow-hidden"
      >
      <!-- Header -->
      <div class="px-5 py-4 border-b border-zinc-800/80 flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <div class="p-2 rounded-xl bg-zinc-800 text-zinc-300">
            <FolderGit2 class="w-5 h-5 text-emerald-400" />
          </div>
          <div>
            <h2 class="text-base font-semibold text-zinc-100">Repositories</h2>
            <p class="text-xs text-zinc-400">Switch workspace or add new project</p>
          </div>
        </div>
        <button
          class="p-2 rounded-xl text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors"
          @click="emit('close')"
        >
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Add Repo Form -->
      <div class="p-4 border-b border-zinc-800 bg-zinc-950/40">
        <form @submit.prevent="handleAdd" class="flex gap-2">
          <input
            v-model="newPath"
            type="text"
            placeholder="Absolute repo path (/Users/...)"
            class="flex-1 px-3.5 py-2.5 bg-zinc-900 border border-zinc-700/80 rounded-xl text-sm text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-emerald-500 transition-colors font-mono text-xs"
          />
          <button
            type="submit"
            :disabled="!newPath.trim() || isAdding"
            class="px-4 py-2.5 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white rounded-xl text-sm font-semibold flex items-center gap-1.5 transition-colors active:scale-95 shrink-0"
          >
            <Plus class="w-4 h-4" />
            <span>Add</span>
          </button>
        </form>
      </div>

      <!-- Repos List -->
      <div class="flex-1 overflow-y-auto p-4 space-y-2">
        <div
          v-if="repos.length === 0"
          class="py-12 text-center text-zinc-500 space-y-2"
        >
          <FolderGit2 class="w-10 h-10 mx-auto stroke-1 text-zinc-600" />
          <p class="text-sm">No repositories registered yet.</p>
          <p class="text-xs text-zinc-600">Enter a project directory path above to get started.</p>
        </div>

        <div
          v-for="repo in repos"
          :key="repo.id"
          :class="[
            'group relative flex items-center justify-between p-3.5 rounded-2xl border transition-all cursor-pointer select-none',
            repo.id === activeRepoId
              ? 'bg-emerald-950/20 border-emerald-500/40'
              : 'bg-zinc-900/60 hover:bg-zinc-800/60 border-zinc-800/80',
          ]"
          @click="emit('select', repo)"
        >
          <div class="flex items-start sm:items-center gap-3 min-w-0 flex-1">
            <div
              :class="[
                'p-2 rounded-xl border shrink-0 mt-0.5 sm:mt-0',
                repo.id === activeRepoId
                  ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
                  : 'bg-zinc-800 border-zinc-700/50 text-zinc-400',
              ]"
            >
              <FolderGit2 class="w-4 h-4" />
            </div>

            <div class="min-w-0 flex-1">
              <div class="font-semibold text-sm text-zinc-100 truncate">
                {{ repo.name }}
              </div>
              <div v-if="repo.branch || (repo.dirtyCount && repo.dirtyCount > 0) || repo.clean" class="mt-1 flex items-center gap-1.5 flex-wrap min-w-0">
                <span
                  v-if="repo.branch"
                  class="inline-flex items-center gap-1 text-[11px] font-mono px-2 py-0.5 rounded-md bg-zinc-800 text-zinc-300 border border-zinc-700/60 shrink-0 max-w-[150px] sm:max-w-[200px]"
                >
                  <GitBranch class="w-3 h-3 text-emerald-400 shrink-0" />
                  <span class="truncate">{{ repo.branch }}</span>
                </span>

                <!-- Dirty count badge -->
                <span
                  v-if="repo.dirtyCount && repo.dirtyCount > 0"
                  class="text-[11px] font-mono font-medium px-2 py-0.5 rounded-md bg-amber-500/10 text-amber-400 border border-amber-500/20 shrink-0"
                >
                  {{ repo.dirtyCount }} changed
                </span>
                <span
                  v-else-if="repo.clean"
                  class="text-[10px] font-mono font-medium px-2 py-0.5 rounded-md bg-zinc-800 text-zinc-400 shrink-0"
                >
                  clean
                </span>
              </div>
              <div class="text-xs text-zinc-500 font-mono truncate mt-1">
                {{ repo.path }}
              </div>
            </div>
          </div>

          <!-- Delete button -->
          <button
            type="button"
            class="p-2 rounded-lg text-zinc-500 hover:text-red-400 hover:bg-red-500/10 transition-colors shrink-0 ml-2"
            title="Remove from list"
            @click.stop="promptDelete(repo)"
          >
            <Trash2 class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>

    <!-- Confirm Remove Modal -->
    <ConfirmModal
      :show="!!repoToDelete"
      title="Remove Repository"
      :message="`Are you sure you want to remove '${repoToDelete?.name}' from Branch Office? This only untracks it from this app; local files are unaffected.`"
      confirm-text="Remove"
      @cancel="repoToDelete = null"
      @confirm="confirmDelete"
    />
  </div>
  </Teleport>
</template>
