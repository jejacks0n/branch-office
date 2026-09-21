<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import type { Repo, RepoStatus, FileStatus, FileDiff, PRStatus, Worktree } from './types'
import { api } from './api'
import RepoSelector from './components/RepoSelector.vue'
import FileList from './components/FileList.vue'
import DiffViewer from './components/DiffViewer.vue'
import CommitDrawer from './components/CommitDrawer.vue'
import SyncModal from './components/SyncModal.vue'
import PrModal from './components/PrModal.vue'
import WorktreeModal from './components/WorktreeModal.vue'
import BranchModal from './components/BranchModal.vue'
import StashModal from './components/StashModal.vue'
import TagModal from './components/TagModal.vue'
import {
  FolderGit2,
  GitBranch,
  GitFork,
  UploadCloud,
  GitPullRequest,
  RefreshCw,
  AlertCircle,
  ChevronDown,
  Archive,
  Tag,
  MoreHorizontal,
} from 'lucide-vue-next'

const repos = ref<Repo[]>([])
const activeRepoId = ref<string>('')
const status = ref<RepoStatus | null>(null)
const prStatus = ref<PRStatus | null>(null)
const worktrees = ref<Worktree[]>([])

const activeDiff = ref<FileDiff | null>(null)
const activeDiffStaged = ref(false)
const activeDiffUntracked = ref(false)
const activeDiffFile = ref<string>('')

const showRepoSelector = ref(false)
const showSyncModal = ref(false)
const showPrModal = ref(false)
const showWorktreeModal = ref(false)
const showBranchModal = ref(false)
const showStashModal = ref(false)
const showTagModal = ref(false)
const showActionsDropdown = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

const loading = ref(false)
const error = ref<string | null>(null)
const sseConnected = ref(false)
const isCommitDrawerOpen = ref(false)

let eventSource: EventSource | null = null
let refreshDebounce: any = null

const activeRepo = computed(() => repos.value.find((r) => r.id === activeRepoId.value))

onMounted(async () => {
  await loadRepos()

  // When switching back to the tab or unlocking phone, ensure SSE is alive
  const handleVisibility = () => {
    if (document.visibilityState === 'visible') {
      if (!sseConnected.value || !eventSource) {
        console.log('[BranchOffice] Tab visible and SSE disconnected; reconnecting...')
        connectSSE()
        refreshStatus(true, 'visibility-reconnect')
      }
    }
  }

  // Dismiss dropdown on outside clicks/taps
  const handleClickOutside = (e: MouseEvent) => {
    if (dropdownRef.value && !dropdownRef.value.contains(e.target as Node)) {
      showActionsDropdown.value = false
    }
  }

  document.addEventListener('visibilitychange', handleVisibility)
  document.addEventListener('pointerdown', handleClickOutside)

  onUnmounted(() => {
    disconnectSSE()
    document.removeEventListener('visibilitychange', handleVisibility)
    document.removeEventListener('pointerdown', handleClickOutside)
  })
})

watch(activeRepoId, () => {
  connectSSE()
  showActionsDropdown.value = false
  showTagModal.value = false
  showStashModal.value = false
  showBranchModal.value = false
  showWorktreeModal.value = false
  showSyncModal.value = false
  showPrModal.value = false
  activeDiff.value = null
})

watch(showWorktreeModal, (open) => {
  if (open) {
    loadWorktrees()
  }
})

// Auto-collapse commit drawer whenever any modal, dropdown, or diff viewer opens
watch(
  [showRepoSelector, showSyncModal, showPrModal, showWorktreeModal, showBranchModal, showStashModal, showTagModal, showActionsDropdown, () => !!activeDiff.value],
  ([repo, sync, pr, wt, branch, stash, tag, dropdown, diff]) => {
    if (repo || sync || pr || wt || branch || stash || tag || dropdown || diff) {
      isCommitDrawerOpen.value = false
    }
  }
)

let currentSSERepoId = ''

function connectSSE() {
  if (!activeRepoId.value) return
  if (eventSource && currentSSERepoId === activeRepoId.value) {
    return
  }
  disconnectSSE()
  currentSSERepoId = activeRepoId.value

  try {
    eventSource = new EventSource(`/api/repos/${activeRepoId.value}/events`)

    eventSource.onopen = () => {
      console.log('[BranchOffice] SSE stream opened')
      sseConnected.value = true
    }

    eventSource.addEventListener('connected', () => {
      console.log('[BranchOffice] SSE connected')
      sseConnected.value = true
    })

    eventSource.addEventListener('change', () => {
      console.log('[BranchOffice] SSE change event received from server')
      sseConnected.value = true
      // Debounce rapid sequential change events
      if (refreshDebounce) clearTimeout(refreshDebounce)
      refreshDebounce = setTimeout(() => {
        refreshStatus(true, 'sse-change')
      }, 150)
    })

    eventSource.onerror = (e) => {
      console.warn('[BranchOffice] SSE error / disconnected', e)
      sseConnected.value = false
    }
  } catch (e) {
    console.error('[BranchOffice] Failed to create EventSource', e)
    sseConnected.value = false
  }
}

function disconnectSSE() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  currentSSERepoId = ''
  sseConnected.value = false
}

async function loadRepos() {
  try {
    loading.value = true
    error.value = null
    const list = await api.getRepos()
    repos.value = list

    // Restore selected repo from localStorage or pick first
    const saved = localStorage.getItem('broffice_active_repo')
    if (saved && list.some((r) => r.id === saved)) {
      activeRepoId.value = saved
    } else if (list.length > 0) {
      activeRepoId.value = list[0].id
    }

    if (activeRepoId.value) {
      await refreshStatus(false, 'initial-load')
      await loadWorktrees()
    }
  } catch (err: any) {
    error.value = err.message || 'Failed to load repositories'
  } finally {
    loading.value = false
  }
}

async function handleSelectRepo(repo: Repo) {
  activeRepoId.value = repo.id
  localStorage.setItem('broffice_active_repo', repo.id)
  showRepoSelector.value = false
  await refreshStatus(false, 'repo-switch')
  await loadWorktrees()
}

async function handleAddRepo(path: string) {
  try {
    loading.value = true
    error.value = null
    const created = await api.addRepo(path)
    await loadRepos()
    await handleSelectRepo(created)
  } catch (err: any) {
    error.value = err.message || 'Failed to add repository'
  } finally {
    loading.value = false
  }
}

async function handleRemoveRepo(repoId: string) {
  try {
    loading.value = true
    await api.removeRepo(repoId)
    await loadRepos()
    if (activeRepoId.value === repoId) {
      activeRepoId.value = repos.value.length > 0 ? repos.value[0].id : ''
      if (activeRepoId.value) {
        await refreshStatus(false, 'repo-remove')
      } else {
        status.value = null
      }
    }
  } catch (err: any) {
    error.value = err.message || 'Failed to remove repository'
  } finally {
    loading.value = false
  }
}

async function refreshStatus(silent = false, reason = 'manual') {
  if (!activeRepoId.value) return
  console.log(`[BranchOffice] refreshStatus: reason=${reason}, silent=${silent}`)
  try {
    if (!silent) {
      loading.value = true
    }
    error.value = null
    const s = await api.getStatus(activeRepoId.value)
    status.value = s

    // If diff viewer is open, refresh its diff content
    if (activeDiff.value && activeDiffFile.value) {
      await loadDiffForFile(activeDiffFile.value, activeDiffStaged.value, activeDiffUntracked.value)
    }
  } catch (err: any) {
    if (!silent) {
      error.value = err.message || 'Failed to fetch status'
    }
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

async function handleViewDiff(file: FileStatus, staged: boolean) {
  activeDiffFile.value = file.path
  activeDiffStaged.value = staged
  activeDiffUntracked.value = file.isUntracked
  await loadDiffForFile(file.path, staged, file.isUntracked)
}

async function loadDiffForFile(filePath: string, staged: boolean, untracked: boolean) {
  if (!activeRepoId.value) return
  try {
    loading.value = true
    const diffs = await api.getDiff(activeRepoId.value, filePath, staged, untracked)
    if (diffs && diffs.length > 0) {
      activeDiff.value = diffs[0]
    } else {
      activeDiff.value = {
        oldPath: filePath,
        newPath: filePath,
        isNew: untracked,
        isDeleted: false,
        isRenamed: false,
        isBinary: false,
        additions: 0,
        deletions: 0,
        hunks: [],
        fileHeader: '',
      }
    }
  } catch (err: any) {
    error.value = err.message || 'Failed to load diff'
  } finally {
    loading.value = false
  }
}

function handleCloseDiff() {
  activeDiff.value = null
  activeDiffFile.value = ''
}

interface DiffNavFile {
  file: FileStatus
  staged: boolean
}

const diffNavFiles = computed<DiffNavFile[]>(() => {
  if (!status.value || !status.value.files) return []
  if (activeDiffStaged.value) {
    return status.value.files
      .filter((f) => f.isStaged)
      .map((f) => ({ file: f, staged: true }))
  }
  return status.value.files
    .filter((f) => f.isUnstaged || f.isUntracked)
    .map((f) => ({ file: f, staged: false }))
})

const currentDiffIndex = computed(() => {
  if (!activeDiffFile.value) return -1
  return diffNavFiles.value.findIndex((item) => item.file.path === activeDiffFile.value)
})

const hasPrevDiff = computed(() => currentDiffIndex.value > 0)
const hasNextDiff = computed(() => {
  if (currentDiffIndex.value === -1) return diffNavFiles.value.length > 0
  return currentDiffIndex.value < diffNavFiles.value.length - 1
})

async function handlePrevDiff() {
  if (!hasPrevDiff.value) return
  const prev = diffNavFiles.value[currentDiffIndex.value - 1]
  if (prev) {
    await handleViewDiff(prev.file, prev.staged)
  }
}

async function handleNextDiff() {
  if (!hasNextDiff.value) return
  const next =
    currentDiffIndex.value >= 0
      ? diffNavFiles.value[currentDiffIndex.value + 1]
      : diffNavFiles.value[0]
  if (next) {
    await handleViewDiff(next.file, next.staged)
  }
}

// Staging actions
async function handleStageFiles(files: string[]) {
  if (!activeRepoId.value) return
  try {
    error.value = null
    await api.stageFiles(activeRepoId.value, files)
    await refreshStatus()
  } catch (err: any) {
    error.value = err.message || 'Failed to stage files'
  }
}

async function handleUnstageFiles(files: string[]) {
  if (!activeRepoId.value) return
  try {
    error.value = null
    await api.unstageFiles(activeRepoId.value, files)
    await refreshStatus()
  } catch (err: any) {
    error.value = err.message || 'Failed to unstage files'
  }
}

async function handleStageAll() {
  if (!activeRepoId.value) return
  try {
    error.value = null
    await api.stageFiles(activeRepoId.value, [], true)
    await refreshStatus()
  } catch (err: any) {
    error.value = err.message || 'Failed to stage all files'
  }
}

async function handleUnstageAll() {
  if (!activeRepoId.value) return
  try {
    error.value = null
    await api.unstageFiles(activeRepoId.value, [], true)
    await refreshStatus()
  } catch (err: any) {
    error.value = err.message || 'Failed to unstage all files'
  }
}

// Hunk actions
async function handleStageHunk(patch: string) {
  if (!activeRepoId.value) return
  try {
    error.value = null
    await api.stageHunk(activeRepoId.value, patch)
    await refreshStatus()
  } catch (err: any) {
    error.value = err.message || 'Failed to stage'
  }
}

async function handleUnstageHunk(patch: string) {
  if (!activeRepoId.value) return
  try {
    error.value = null
    await api.unstageHunk(activeRepoId.value, patch)
    await refreshStatus()
  } catch (err: any) {
    error.value = err.message || 'Failed to unstage'
  }
}

// Discard actions
async function handleDiscardFiles(files: string[]) {
  if (!activeRepoId.value) return
  try {
    error.value = null
    await api.discardFiles(activeRepoId.value, files)
    await refreshStatus()
  } catch (err: any) {
    error.value = err.message || 'Failed to discard changes'
  }
}

async function handleDiscardAll() {
  if (!activeRepoId.value) return
  try {
    error.value = null
    await api.discardFiles(activeRepoId.value, [], true)
    await refreshStatus()
  } catch (err: any) {
    error.value = err.message || 'Failed to discard all changes'
  }
}

async function handleDiscardHunk(patch: string) {
  if (!activeRepoId.value) return
  try {
    error.value = null
    await api.discardHunk(activeRepoId.value, patch)
    await refreshStatus()
  } catch (err: any) {
    error.value = err.message || 'Failed to discard'
  }
}

// Commit
async function handleCommit(payload: { message: string; amend: boolean }) {
  if (!activeRepoId.value) return
  try {
    error.value = null
    await api.commit(activeRepoId.value, payload.message, payload.amend)
    await refreshStatus(false, 'commit')
  } catch (err: any) {
    error.value = err.message || 'Failed to commit'
  }
}

// Push / Sync
async function handleOpenSync() {
  if (!activeRepoId.value) return
  showSyncModal.value = true
}

async function handlePush(options: { forceWithLease: boolean; setUpstream: boolean }) {
  if (!activeRepoId.value) return
  try {
    loading.value = true
    error.value = null
    await api.push(activeRepoId.value, options.forceWithLease, options.setUpstream)
    showSyncModal.value = false
    await refreshStatus()
  } catch (err: any) {
    error.value = err.message || 'Failed to push'
  } finally {
    loading.value = false
  }
}

// PR
async function handleOpenPR() {
  if (!activeRepoId.value) return
  try {
    loading.value = true
    error.value = null
    prStatus.value = await api.getPRStatus(activeRepoId.value)
    showPrModal.value = true
  } catch (err: any) {
    error.value = err.message || 'Failed to check PR status'
  } finally {
    loading.value = false
  }
}

async function handleCreatePR(data: { title: string; body: string; draft: boolean }) {
  if (!activeRepoId.value) return
  try {
    loading.value = true
    error.value = null
    await api.createPR(activeRepoId.value, data.title, data.body, data.draft)
    prStatus.value = await api.getPRStatus(activeRepoId.value)
  } catch (err: any) {
    error.value = err.message || 'Failed to create PR'
  } finally {
    loading.value = false
  }
}

// Worktree actions
async function loadWorktrees() {
  if (!activeRepoId.value) return
  try {
    const list = await api.getWorktrees(activeRepoId.value)
    worktrees.value = list
  } catch {
    worktrees.value = []
  }
}

async function handleSwitchWorktree(targetPath: string) {
  try {
    loading.value = true
    error.value = null
    const repo = await api.addRepo(targetPath)
    await loadRepos()
    await handleSelectRepo(repo)
    showWorktreeModal.value = false
  } catch (err: any) {
    error.value = err.message || 'Failed to switch worktree'
  } finally {
    loading.value = false
  }
}

async function handleAddWorktree(data: { path: string; branch: string; createBranch: boolean }) {
  if (!activeRepoId.value) return
  try {
    loading.value = true
    error.value = null
    const created = await api.addWorktree(activeRepoId.value, data.path, data.branch, data.createBranch)
    await loadRepos()
    const target = repos.value.find((r) => r.path === created.path)
    if (target) {
      await handleSelectRepo(target)
    } else {
      await refreshStatus()
    }
    showWorktreeModal.value = false
  } catch (err: any) {
    error.value = err.message || 'Failed to create worktree'
  } finally {
    loading.value = false
  }
}

async function handleRemoveWorktree(targetPath: string) {
  if (!activeRepoId.value) return
  try {
    loading.value = true
    error.value = null
    await api.removeWorktree(activeRepoId.value, targetPath)
    await loadWorktrees()
    await loadRepos()
  } catch (err: any) {
    error.value = err.message || 'Failed to remove worktree'
  } finally {
    loading.value = false
  }
}

async function handleBranchSwitched(branch: string) {
  if (status.value) {
    status.value.branch = branch
  }
  await refreshStatus(true, 'branch-switched')
  await loadRepos()
}

async function handleBranchCreated(branch: string) {
  if (status.value) {
    status.value.branch = branch
  }
  await refreshStatus(true, 'branch-created')
  await loadRepos()
}
</script>

<template>
  <div class="h-full w-full bg-[#09090b] text-zinc-100 flex flex-col font-sans overflow-hidden">
    <!-- Top Navigation Bar -->
    <header
      class="pt-safe px-4 pb-3.5 bg-zinc-900/90 backdrop-blur-md border-b border-zinc-800/80 shrink-0 z-30 flex items-center justify-between touch-none"
    >
      <!-- Repository Selector Trigger -->
      <button
        type="button"
        class="flex items-center gap-2 px-3 py-1.5 rounded-xl bg-zinc-800/80 hover:bg-zinc-700/80 border border-zinc-700/60 transition-all min-w-0 shrink mr-2 max-w-xs sm:max-w-sm md:max-w-md text-left active:scale-95"
        @click="showRepoSelector = true"
      >
        <FolderGit2 class="w-4 h-4 text-emerald-400 shrink-0" />
        <span class="text-xs font-semibold text-zinc-200 truncate font-mono min-w-0">
          {{ activeRepo ? activeRepo.name : 'Select Repository' }}
        </span>
        <ChevronDown class="w-3.5 h-3.5 text-zinc-400 shrink-0" />
      </button>

      <!-- Action Icons Area -->
      <div class="flex items-center gap-1.5 shrink-0">
        <!-- Sync / Push Modal Trigger (Always visible) -->
        <button
          v-if="status"
          type="button"
          :class="[
            'p-2 rounded-xl border transition-all active:scale-95 shrink-0',
            status.ahead > 0
              ? 'bg-sky-500/10 border-sky-500/30 text-sky-400'
              : 'bg-zinc-800/80 border-zinc-700/50 text-zinc-400 hover:text-zinc-200',
          ]"
          title="Push to remote"
          @click="handleOpenSync"
        >
          <UploadCloud class="w-4 h-4 shrink-0" />
        </button>

        <!-- PR Trigger (Always visible) -->
        <button
          v-if="status"
          type="button"
          class="p-2 rounded-xl bg-zinc-800/80 border border-zinc-700/50 text-zinc-400 hover:text-purple-400 hover:bg-purple-500/10 transition-all active:scale-95 shrink-0"
          title="GitHub Pull Request"
          @click="handleOpenPR"
        >
          <GitPullRequest class="w-4 h-4 shrink-0" />
        </button>

        <!-- Secondary Icons on wider screens (550px and up) -->
        <div class="hidden min-[550px]:flex items-center gap-1.5 shrink-0">
          <!-- Worktrees Trigger -->
          <button
            v-if="status"
            type="button"
            :class="[
              'p-2 rounded-xl border transition-all active:scale-95 flex items-center gap-1 shrink-0',
              worktrees.length > 1
                ? 'bg-teal-500/10 border-teal-500/30 text-teal-400'
                : 'bg-zinc-800/80 border-zinc-700/50 text-zinc-400 hover:text-teal-400',
            ]"
            title="Git Worktrees"
            @click="showWorktreeModal = true"
          >
            <GitFork class="w-4 h-4 shrink-0" />
            <span v-if="worktrees.length > 1" class="text-[10px] font-mono font-bold">{{ worktrees.length }}</span>
          </button>

          <!-- Stashes Trigger -->
          <button
            v-if="status"
            type="button"
            :class="[
              'p-2 rounded-xl border transition-all active:scale-95 flex items-center gap-1 shrink-0',
              (status.stashCount ?? 0) > 0
                ? 'bg-amber-500/10 border-amber-500/30 text-amber-400'
                : 'bg-zinc-800/80 border-zinc-700/50 text-zinc-400 hover:text-amber-400',
            ]"
            title="Git Stashes"
            @click="showStashModal = true"
          >
            <Archive class="w-4 h-4 shrink-0" />
            <span v-if="(status.stashCount ?? 0) > 0" class="text-[10px] font-mono font-bold">{{ status.stashCount }}</span>
          </button>

          <!-- Tags Trigger -->
          <button
            v-if="status"
            type="button"
            :class="[
              'p-2 rounded-xl border transition-all active:scale-95 flex items-center gap-1 shrink-0',
              (status.tagCount ?? 0) > 0
                ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
                : 'bg-zinc-800/80 border-zinc-700/50 text-zinc-400 hover:text-emerald-400',
            ]"
            title="Git Tags"
            @click="showTagModal = true"
          >
            <Tag class="w-4 h-4 shrink-0" />
            <span v-if="(status.tagCount ?? 0) > 0" class="text-[10px] font-mono font-bold">{{ status.tagCount }}</span>
          </button>

          <!-- Refresh Button -->
          <button
            type="button"
            class="p-2 rounded-xl bg-zinc-800/80 border border-zinc-700/50 text-zinc-400 hover:text-zinc-200 transition-all active:scale-95 shrink-0"
            :disabled="loading"
            title="Refresh Git status"
            @click="() => refreshStatus(false, 'refresh-button')"
          >
            <RefreshCw :class="['w-4 h-4 shrink-0', loading ? 'animate-spin text-emerald-400' : '']" />
          </button>
        </div>

        <!-- Actions Dropdown on narrow screens (below 550px) -->
        <div class="relative min-[550px]:hidden shrink-0" ref="dropdownRef">
          <button
            type="button"
            :class="[
              'p-2 rounded-xl border transition-all active:scale-95 relative flex items-center justify-center shrink-0',
              showActionsDropdown
                ? 'bg-zinc-700/90 border-zinc-600 text-white'
                : 'bg-zinc-800/80 border-zinc-700/50 text-zinc-400 hover:text-zinc-200'
            ]"
            title="More actions"
            aria-label="More actions"
            @click="showActionsDropdown = !showActionsDropdown"
          >
            <MoreHorizontal class="w-4 h-4" />
            <span
              v-if="(status?.stashCount ?? 0) > 0 || (status?.tagCount ?? 0) > 0"
              class="absolute -top-1 -right-1 w-2 h-2 rounded-full bg-emerald-400 ring-2 ring-zinc-900"
            />
          </button>

          <!-- Dropdown Menu -->
          <Transition
            enter-active-class="transition duration-100 ease-out"
            enter-from-class="transform scale-95 opacity-0"
            enter-to-class="transform scale-100 opacity-100"
            leave-active-class="transition duration-75 ease-in"
            leave-from-class="transform scale-100 opacity-100"
            leave-to-class="transform scale-95 opacity-0"
          >
            <div
              v-if="showActionsDropdown"
              class="absolute right-0 top-full mt-2 w-48 py-1.5 bg-zinc-900/95 backdrop-blur-xl border border-zinc-800/90 rounded-2xl shadow-2xl z-50 flex flex-col divide-y divide-zinc-800/60 touch-auto font-sans"
            >
              <!-- Secondary Git Actions -->
              <div class="p-1 space-y-0.5">
                <!-- Worktrees -->
                <button
                  v-if="status"
                  type="button"
                  class="w-full flex items-center justify-between px-3 py-2 rounded-xl text-xs font-medium text-zinc-200 hover:bg-zinc-800/80 active:bg-zinc-800 transition-colors"
                  @click="showActionsDropdown = false; showWorktreeModal = true"
                >
                  <div class="flex items-center gap-2.5 min-w-0">
                    <GitFork class="w-4 h-4 text-teal-400 shrink-0" />
                    <span class="truncate">Worktrees</span>
                  </div>
                  <span
                    v-if="worktrees.length > 1"
                    class="px-1.5 py-0.5 text-[10px] font-mono font-bold bg-teal-500/20 text-teal-400 rounded-md border border-teal-500/30 shrink-0"
                  >
                    {{ worktrees.length }}
                  </span>
                </button>

                <!-- Stashes -->
                <button
                  v-if="status"
                  type="button"
                  class="w-full flex items-center justify-between px-3 py-2 rounded-xl text-xs font-medium text-zinc-200 hover:bg-zinc-800/80 active:bg-zinc-800 transition-colors"
                  @click="showActionsDropdown = false; showStashModal = true"
                >
                  <div class="flex items-center gap-2.5 min-w-0">
                    <Archive class="w-4 h-4 text-amber-400 shrink-0" />
                    <span class="truncate">Stashes</span>
                  </div>
                  <span
                    v-if="(status.stashCount ?? 0) > 0"
                    class="px-1.5 py-0.5 text-[10px] font-mono font-bold bg-amber-500/20 text-amber-400 rounded-md border border-amber-500/30 shrink-0"
                  >
                    {{ status.stashCount }}
                  </span>
                </button>

                <!-- Tags -->
                <button
                  v-if="status"
                  type="button"
                  class="w-full flex items-center justify-between px-3 py-2 rounded-xl text-xs font-medium text-zinc-200 hover:bg-zinc-800/80 active:bg-zinc-800 transition-colors"
                  @click="showActionsDropdown = false; showTagModal = true"
                >
                  <div class="flex items-center gap-2.5 min-w-0">
                    <Tag class="w-4 h-4 text-emerald-400 shrink-0" />
                    <span class="truncate">Tags</span>
                  </div>
                  <span
                    v-if="(status.tagCount ?? 0) > 0"
                    class="px-1.5 py-0.5 text-[10px] font-mono font-bold bg-emerald-500/20 text-emerald-400 rounded-md border border-emerald-500/30 shrink-0"
                  >
                    {{ status.tagCount }}
                  </span>
                </button>
              </div>

              <!-- Refresh Status -->
              <div class="p-1">
                <button
                  type="button"
                  class="w-full flex items-center justify-between px-3 py-2 rounded-xl text-xs font-medium text-zinc-200 hover:bg-zinc-800/80 active:bg-zinc-800 transition-colors disabled:opacity-50"
                  :disabled="loading"
                  @click="showActionsDropdown = false; refreshStatus(false, 'dropdown-refresh')"
                >
                  <div class="flex items-center gap-2.5 min-w-0">
                    <RefreshCw :class="['w-4 h-4 text-zinc-400 shrink-0', loading ? 'animate-spin text-emerald-400' : '']" />
                    <span class="truncate">Refresh Status</span>
                  </div>
                </button>
              </div>
            </div>
          </Transition>
        </div>
      </div>
    </header>

    <!-- Error Banner -->
    <div
      v-if="error"
      class="mx-4 mt-3 p-3.5 rounded-2xl bg-red-500/10 border border-red-500/20 text-red-400 text-xs flex items-start justify-between gap-2 shadow-sm shrink-0"
    >
      <div class="flex items-start gap-2">
        <AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
        <span class="font-mono whitespace-pre-wrap">{{ error }}</span>
      </div>
      <button class="text-red-400 hover:text-red-200 shrink-0 font-bold" @click="error = null">
        ✕
      </button>
    </div>

    <!-- Main Workspace Area -->
    <main
      :class="[
        'flex-1 overflow-y-auto overscroll-y-contain w-full p-4 sm:px-6 md:px-8 space-y-4 transition-[padding-bottom] duration-200',
        isCommitDrawerOpen ? 'pb-96 sm:pb-[26rem]' : 'pb-36'
      ]"
    >
      <!-- No active repo -->
      <div
        v-if="!activeRepo"
        class="py-20 text-center space-y-4 bg-zinc-900/40 rounded-3xl border border-zinc-800/60 p-6"
      >
        <FolderGit2 class="w-12 h-12 mx-auto text-zinc-600" />
        <div class="space-y-1">
          <h2 class="text-base font-semibold text-zinc-200">No Repository Selected</h2>
          <p class="text-xs text-zinc-500">Pick a registered repository or add a path on your machine.</p>
        </div>
        <button
          type="button"
          class="px-4 py-2.5 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs rounded-xl shadow-md transition-all active:scale-95"
          @click="showRepoSelector = true"
        >
          Manage Repositories
        </button>
      </div>

      <!-- Active Repo Git Control View -->
      <template v-else-if="status">
        <!-- Branch Header Status Pill -->
        <div class="flex items-center justify-between p-3.5 rounded-2xl bg-zinc-900/60 border border-zinc-800/80">
          <!-- Interactive Branch Switcher Trigger -->
          <button
            type="button"
            class="flex items-center gap-2.5 min-w-0 pr-2 group text-left cursor-pointer hover:opacity-90 active:scale-[0.98] transition-all"
            title="Switch or create branches"
            @click="showBranchModal = true"
          >
            <div class="p-1.5 rounded-lg bg-emerald-500/10 text-emerald-400 group-hover:bg-emerald-500/20 transition-colors shrink-0">
              <GitBranch class="w-4 h-4" />
            </div>
            <div class="min-w-0">
              <div class="flex items-center gap-1.5">
                <span class="text-xs font-semibold text-zinc-200 group-hover:text-emerald-300 font-mono truncate block">
                  {{ status.branch }}
                </span>
                <ChevronDown class="w-3.5 h-3.5 text-zinc-500 group-hover:text-zinc-300 shrink-0" />
              </div>
              <span v-if="status.upstream" class="text-[11px] text-zinc-500 font-mono truncate block">
                {{ status.upstream }}
              </span>
            </div>
          </button>

          <div class="flex items-center gap-2 shrink-0">
            <!-- Worktree quick pill -->
            <button
              v-if="worktrees.length > 0"
              type="button"
              class="px-2 py-0.5 rounded-md bg-teal-500/10 text-teal-400 border border-teal-500/20 text-[11px] font-mono font-medium flex items-center gap-1 active:scale-95"
              title="View and switch Git Worktrees"
              @click="showWorktreeModal = true"
            >
              <GitFork class="w-3 h-3" />
              <span>{{ worktrees.length }} {{ worktrees.length === 1 ? 'worktree' : 'worktrees' }}</span>
            </button>

            <!-- Stash quick pill -->
            <button
              v-if="(status.stashCount ?? 0) > 0"
              type="button"
              class="px-2 py-0.5 rounded-md bg-amber-500/10 text-amber-400 border border-amber-500/20 text-[11px] font-mono font-medium flex items-center gap-1 active:scale-95"
              title="View Git Stashes"
              @click="showStashModal = true"
            >
              <Archive class="w-3 h-3" />
              <span>{{ status.stashCount }} {{ status.stashCount === 1 ? 'stash' : 'stashes' }}</span>
            </button>

            <!-- Ahead / Behind badge -->
            <div class="flex items-center gap-1.5 text-[11px] font-mono">
              <span
                v-if="status.ahead > 0"
                class="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium"
              >
                +{{ status.ahead }}
              </span>
              <span
                v-if="status.behind > 0"
                class="px-2 py-0.5 rounded-md bg-zinc-800 text-zinc-400 border border-zinc-700/60 font-medium"
              >
                -{{ status.behind }}
              </span>
            </div>
          </div>
        </div>

        <!-- File List Groupings -->
        <FileList
          :status="status"
          @stage-files="handleStageFiles"
          @unstage-files="handleUnstageFiles"
          @stage-all="handleStageAll"
          @unstage-all="handleUnstageAll"
          @discard-files="handleDiscardFiles"
          @discard-all="handleDiscardAll"
          @view-diff="handleViewDiff"
        />
      </template>
    </main>

    <!-- Floating Commit Bottom Drawer -->
    <CommitDrawer
      v-if="status"
      :repo-id="activeRepoId"
      :staged-count="status.stagedCount"
      :has-head="status.hasHead"
      :last-commit-message="status.lastCommitMessage"
      :open="isCommitDrawerOpen"
      @update:open="(open) => (isCommitDrawerOpen = open)"
      @commit="handleCommit"
    />

    <!-- Modals & Drawers -->
    <RepoSelector
      :show="showRepoSelector"
      :repos="repos"
      :active-repo-id="activeRepoId"
      @select="handleSelectRepo"
      @add="handleAddRepo"
      @remove="handleRemoveRepo"
      @close="showRepoSelector = false"
    />

    <SyncModal
      v-if="status"
      :show="showSyncModal"
      :branch="status.branch"
      :upstream="status.upstream"
      :ahead="status.ahead"
      :behind="status.behind"
      @push="handlePush"
      @close="showSyncModal = false"
    />

    <PrModal
      v-if="status"
      :show="showPrModal"
      :status="prStatus"
      :branch="status.branch"
      @create-pr="handleCreatePR"
      @close="showPrModal = false"
    />

    <DiffViewer
      :show="!!activeDiff"
      :file-diff="activeDiff"
      :staged="activeDiffStaged"
      :untracked="activeDiffUntracked"
      :has-prev="hasPrevDiff"
      :has-next="hasNextDiff"
      :file-index="currentDiffIndex"
      :total-files="diffNavFiles.length"
      @prev="handlePrevDiff"
      @next="handleNextDiff"
      @stage-hunk="handleStageHunk"
      @unstage-hunk="handleUnstageHunk"
      @discard-hunk="handleDiscardHunk"
      @close="handleCloseDiff"
    />

    <WorktreeModal
      v-if="activeRepo"
      :show="showWorktreeModal"
      :worktrees="worktrees"
      :current-path="activeRepo.path"
      :repo-name="activeRepo.name"
      @switch="handleSwitchWorktree"
      @add="handleAddWorktree"
      @remove="handleRemoveWorktree"
      @close="showWorktreeModal = false"
    />

    <BranchModal
      v-if="activeRepo && status"
      :show="showBranchModal"
      :repo-id="activeRepoId"
      :current-branch="status.branch"
      :repo-name="activeRepo.name"
      @switched="handleBranchSwitched"
      @created="handleBranchCreated"
      @close="showBranchModal = false"
    />

    <StashModal
      v-if="activeRepo && status"
      :show="showStashModal"
      :repo-id="activeRepoId"
      :repo-name="activeRepo.name"
      :change-count="status.files.length"
      @changed="() => refreshStatus(true, 'stash-changed')"
      @close="showStashModal = false"
    />

    <TagModal
      v-if="activeRepo && status"
      :show="showTagModal"
      :repo-id="activeRepoId"
      :repo-name="activeRepo.name"
      @changed="() => refreshStatus(true, 'tag-changed')"
      @close="showTagModal = false"
    />
  </div>
</template>
