<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import type { CommitItem, FileDiff } from '../types'
import { api } from '../api'
import {
  History,
  Search,
  X,
  Copy,
  ChevronDown,
  ChevronUp,
  Loader2,
  AlertCircle,
  FileCode,
  Tag,
} from 'lucide-vue-next'

const props = defineProps<{
  show: boolean
  repoId: string
  repoName: string
  currentBranch: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'view-commit-diff', payload: { diffs: FileDiff[]; fileIndex: number; commit: CommitItem }): void
}>()

const commits = ref<CommitItem[]>([])
const loading = ref(false)
const loadingMore = ref(false)
const hasMore = ref(true)
const modalError = ref<string | null>(null)
const searchQuery = ref('')
let searchDebounce: any = null

// Expanded commit details & diffs caching
const expandedHash = ref<string | null>(null)
const loadingDetails = ref(false)
const commitDiffsCache = ref<Record<string, FileDiff[]>>({})
const copiedHash = ref<string | null>(null)

watch(
  () => props.show,
  async (open) => {
    if (open && props.repoId) {
      searchQuery.value = ''
      modalError.value = null
      expandedHash.value = null
      commitDiffsCache.value = {}
      await loadCommits(false)
    }
  }
)

async function loadCommits(append = false) {
  if (!props.repoId) return
  if (append) {
    loadingMore.value = true
  } else {
    loading.value = true
    commits.value = []
  }
  modalError.value = null

  try {
    const skip = append ? commits.value.length : 0
    const fetched = await api.getCommits(props.repoId, {
      limit: 30,
      skip,
      search: searchQuery.value.trim() || undefined,
    })

    if (append) {
      commits.value = [...commits.value, ...fetched]
    } else {
      commits.value = fetched
    }

    // If fetched fewer than 30, we've reached the end
    hasMore.value = fetched.length >= 30
  } catch (err: any) {
    modalError.value = err.message || 'Failed to fetch commit log'
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

function handleSearchInput() {
  if (searchDebounce) clearTimeout(searchDebounce)
  searchDebounce = setTimeout(() => {
    expandedHash.value = null
    loadCommits(false)
  }, 250)
}

function clearSearch() {
  searchQuery.value = ''
  expandedHash.value = null
  loadCommits(false)
}

async function toggleExpand(commit: CommitItem) {
  if (expandedHash.value === commit.hash) {
    expandedHash.value = null
    return
  }

  expandedHash.value = commit.hash

  if (!commitDiffsCache.value[commit.hash]) {
    loadingDetails.value = true
    try {
      const details = await api.getCommit(props.repoId, commit.hash)
      commitDiffsCache.value[commit.hash] = details.diffs || []
    } catch (err: any) {
      modalError.value = err.message || `Failed to load details for ${commit.shortHash}`
    } finally {
      loadingDetails.value = false
    }
  }
}

async function copyHash(hash: string) {
  try {
    await navigator.clipboard.writeText(hash)
    copiedHash.value = hash
    setTimeout(() => {
      if (copiedHash.value === hash) {
        copiedHash.value = null
      }
    }, 1500)
  } catch {
    // fallback or ignore
  }
}

function getInitials(name: string): string {
  if (!name) return '?'
  const parts = name.trim().split(/\s+/)
  if (parts.length >= 2) {
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
  }
  return name.slice(0, 2).toUpperCase()
}

function getRefKind(ref: string): 'head' | 'remote' | 'tag' | 'other' {
  if (ref.includes('HEAD') || ref.startsWith('main') || ref.startsWith('master')) {
    return 'head'
  }
  if (ref.startsWith('tag:')) {
    return 'tag'
  }
  if (ref.includes('/')) {
    return 'remote'
  }
  return 'other'
}

function cleanRefName(ref: string): string {
  return ref.replace(/^tag:\s*/, '')
}

function viewFileDiff(diffs: FileDiff[], fileIndex: number, commit: CommitItem) {
  emit('view-commit-diff', { diffs, fileIndex, commit })
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="show"
      class="viewport-fixed modal-overlay z-50 bg-black/70 backdrop-blur-sm animate-fade-in"
      @click.self="emit('close')"
    >
      <div
        class="modal-panel max-w-2xl bg-zinc-900 border border-zinc-800 rounded-3xl shadow-2xl flex flex-col overflow-hidden text-zinc-100"
      >
        <!-- Modal Header -->
        <div class="px-5 py-4 border-b border-zinc-800/80 flex items-center justify-between shrink-0 bg-zinc-900/90">
          <div class="flex items-center gap-2.5 min-w-0">
            <div class="p-2 rounded-xl bg-purple-500/10 border border-purple-500/20 text-purple-400 shrink-0">
              <History class="w-5 h-5" />
            </div>
            <div class="min-w-0">
              <h2 class="text-base font-semibold text-zinc-100 truncate">Commit History</h2>
              <p class="text-xs text-zinc-400 truncate">
                <span class="font-mono text-zinc-300">{{ repoName }}</span>
                <span v-if="currentBranch" class="mx-1.5 text-zinc-600">•</span>
                <span v-if="currentBranch" class="font-mono text-purple-400">{{ currentBranch }}</span>
              </p>
            </div>
          </div>

          <button
            type="button"
            class="p-2 rounded-xl text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors shrink-0 ml-2"
            title="Close (Esc)"
            @click="emit('close')"
          >
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Search Bar -->
        <div class="p-3 border-b border-zinc-800/60 bg-zinc-950/40 shrink-0">
          <div class="relative flex items-center">
            <Search class="w-4 h-4 text-zinc-400 absolute left-3 pointer-events-none" />
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Search commit messages, authors, or hashes..."
              class="w-full pl-9 pr-9 py-2 rounded-xl bg-zinc-900 border border-zinc-700/60 text-xs text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-purple-500/60 focus:ring-1 focus:ring-purple-500/40 transition-all"
              @input="handleSearchInput"
            />
            <button
              v-if="searchQuery"
              type="button"
              class="absolute right-2.5 p-1 rounded-md text-zinc-400 hover:text-zinc-200 transition-colors"
              title="Clear search"
              @click="clearSearch"
            >
              <X class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <!-- Inline Error Banner -->
        <div
          v-if="modalError"
          class="mx-4 mt-3 p-3 rounded-2xl bg-red-500/10 border border-red-500/20 text-red-400 text-xs flex items-center gap-2 shrink-0"
        >
          <AlertCircle class="w-4 h-4 shrink-0" />
          <span class="truncate">{{ modalError }}</span>
        </div>

        <!-- Commit List Content -->
        <div class="flex-1 overflow-y-auto p-3 sm:p-4 space-y-2.5">
          <!-- Initial Loading State -->
          <div v-if="loading && commits.length === 0" class="py-16 text-center text-zinc-500 text-xs flex flex-col items-center gap-2">
            <Loader2 class="w-5 h-5 animate-spin text-purple-400" />
            <span>Loading commits...</span>
          </div>

          <!-- Empty State -->
          <div
            v-else-if="!loading && commits.length === 0"
            class="py-16 text-center text-zinc-500 text-xs flex flex-col items-center gap-2"
          >
            <History class="w-6 h-6 text-zinc-600" />
            <span>{{ searchQuery ? 'No commits match your search query.' : 'No commits found in this repository.' }}</span>
          </div>

          <!-- Commit Cards -->
          <div
            v-for="commit in commits"
            :key="commit.hash"
            :class="[
              'rounded-2xl border transition-all duration-150 overflow-hidden',
              expandedHash === commit.hash
                ? 'bg-zinc-900/90 border-purple-500/40 shadow-lg'
                : 'bg-zinc-950/50 border-zinc-800/80 hover:border-zinc-700/80',
            ]"
          >
            <!-- Commit Summary Header (Tappable) -->
            <div
              class="p-3.5 flex items-start gap-3 cursor-pointer select-none"
              @click="toggleExpand(commit)"
            >
              <!-- Author Initials Circle -->
              <div
                class="w-8 h-8 rounded-full bg-zinc-800/90 border border-zinc-700/60 flex items-center justify-center text-[11px] font-semibold text-zinc-300 shrink-0 select-none mt-0.5"
                :title="`${commit.author} <${commit.email}>`"
              >
                {{ getInitials(commit.author) }}
              </div>

              <!-- Main Info Column -->
              <div class="min-w-0 flex-1 space-y-1">
                <!-- Subject Line -->
                <div class="flex items-center justify-between gap-2">
                  <h3 class="text-xs sm:text-sm font-medium text-zinc-100 truncate leading-snug">
                    {{ commit.subject }}
                  </h3>
                  <!-- Expand Icon -->
                  <div class="text-zinc-500 shrink-0 ml-1">
                    <component
                      :is="expandedHash === commit.hash ? ChevronUp : ChevronDown"
                      class="w-4 h-4"
                    />
                  </div>
                </div>

                <!-- Author & Time Row -->
                <div class="flex items-center justify-between text-xs text-zinc-400 gap-2">
                  <span class="truncate">{{ commit.author }}</span>
                  <span class="text-zinc-500 text-[11px] shrink-0" :title="commit.date">
                    {{ commit.relativeDate }}
                  </span>
                </div>

                <!-- Badges & Hash Row -->
                <div class="flex items-center gap-1.5 pt-0.5 flex-wrap">
                  <!-- Monospace Short Hash with 1-tap Copy -->
                  <div class="flex items-center bg-zinc-800/90 rounded-md border border-zinc-700/60 shrink-0 overflow-hidden">
                    <span class="font-mono text-[11px] text-zinc-400 px-1.5 py-0.5 select-all">
                      {{ commit.shortHash }}
                    </span>
                    <button
                      type="button"
                      class="px-1.5 py-0.5 border-l border-zinc-700/60 text-zinc-400 hover:text-zinc-200 transition-colors flex items-center gap-1"
                      title="Copy full commit hash"
                      @click.stop="copyHash(commit.hash)"
                    >
                      <span v-if="copiedHash === commit.hash" class="text-[10px] text-purple-400 font-sans font-medium">Copied</span>
                      <Copy v-else class="w-3 h-3" />
                    </button>
                  </div>

                  <!-- Ref Decorations -->
                  <template v-for="refName in commit.refs" :key="refName">
                    <!-- HEAD / Local Branch Badge -->
                    <span
                      v-if="getRefKind(refName) === 'head'"
                      class="text-[10px] font-mono px-1.5 py-0.5 rounded-md bg-emerald-500/15 text-emerald-400 border border-emerald-500/30 shrink-0 truncate max-w-[140px]"
                      :title="refName"
                    >
                      {{ refName }}
                    </span>

                    <!-- Remote Branch Badge -->
                    <span
                      v-else-if="getRefKind(refName) === 'remote'"
                      class="text-[10px] font-mono px-1.5 py-0.5 rounded-md bg-sky-500/15 text-sky-400 border border-sky-500/30 shrink-0 truncate max-w-[140px]"
                      :title="refName"
                    >
                      {{ refName }}
                    </span>

                    <!-- Tag Badge -->
                    <span
                      v-else-if="getRefKind(refName) === 'tag'"
                      class="flex items-center gap-1 text-[10px] font-mono px-1.5 py-0.5 rounded-md bg-amber-500/15 text-amber-400 border border-amber-500/30 shrink-0 truncate max-w-[140px]"
                      :title="refName"
                    >
                      <Tag class="w-2.5 h-2.5 shrink-0" />
                      <span class="truncate">{{ cleanRefName(refName) }}</span>
                    </span>

                    <!-- Other Ref -->
                    <span
                      v-else
                      class="text-[10px] font-mono px-1.5 py-0.5 rounded-md bg-zinc-800 text-zinc-400 border border-zinc-700/60 shrink-0 truncate max-w-[140px]"
                    >
                      {{ refName }}
                    </span>
                  </template>
                </div>
              </div>
            </div>

            <!-- Expanded Details (Body & Changed Files) -->
            <div
              v-if="expandedHash === commit.hash"
              class="border-t border-zinc-800/80 bg-zinc-950/70 p-3.5 space-y-3"
            >
              <!-- Commit Body if available -->
              <div v-if="commit.body" class="p-3 rounded-xl bg-zinc-900/90 border border-zinc-800/80 text-xs font-mono text-zinc-300 whitespace-pre-wrap leading-relaxed select-text">
                {{ commit.body }}
              </div>

              <!-- Full Hash & Author Info -->
              <div class="p-2.5 rounded-xl bg-zinc-900/60 border border-zinc-800/60 text-xs space-y-1">
                <div class="flex items-center justify-between text-zinc-400 text-[11px]">
                  <span>Full Hash:</span>
                  <span class="font-mono text-zinc-300 truncate max-w-[280px] sm:max-w-md select-all">{{ commit.hash }}</span>
                </div>
                <div class="flex items-center justify-between text-zinc-400 text-[11px]">
                  <span>Date:</span>
                  <span class="font-mono text-zinc-300 truncate">{{ commit.date }}</span>
                </div>
              </div>

              <!-- Changed Files List -->
              <div class="space-y-2">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-zinc-400 font-medium">Changed Files</span>
                  <button
                    v-if="commitDiffsCache[commit.hash] && commitDiffsCache[commit.hash].length > 0"
                    type="button"
                    class="flex items-center gap-1 text-xs text-purple-400 hover:text-purple-300 font-medium transition-colors"
                    @click="viewFileDiff(commitDiffsCache[commit.hash], 0, commit)"
                  >
                    <FileCode class="w-3.5 h-3.5" />
                    <span>View All Changes ({{ commitDiffsCache[commit.hash].length }})</span>
                  </button>
                </div>

                <!-- Loading Diffs -->
                <div v-if="loadingDetails" class="py-4 text-center text-xs text-zinc-500 flex items-center justify-center gap-2">
                  <Loader2 class="w-4 h-4 animate-spin text-purple-400" />
                  <span>Loading commit changes...</span>
                </div>

                <!-- Files List -->
                <div
                  v-else-if="commitDiffsCache[commit.hash]"
                  class="space-y-1.5"
                >
                  <div
                    v-for="(diff, fIdx) in commitDiffsCache[commit.hash]"
                    :key="diff.newPath || diff.oldPath"
                    class="p-2.5 rounded-xl bg-zinc-900/80 border border-zinc-800/80 hover:border-zinc-700/80 flex items-center justify-between gap-2 cursor-pointer transition-all active:scale-[0.99]"
                    @click="viewFileDiff(commitDiffsCache[commit.hash], fIdx, commit)"
                  >
                    <!-- File Path & Change Type Badge -->
                    <div class="flex items-center gap-2 min-w-0">
                      <span
                        v-if="diff.isNew"
                        class="text-[9px] uppercase font-bold px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 shrink-0"
                      >
                        New
                      </span>
                      <span
                        v-else-if="diff.isDeleted"
                        class="text-[9px] uppercase font-bold px-1.5 py-0.5 rounded bg-red-500/20 text-red-400 border border-red-500/30 shrink-0"
                      >
                        Del
                      </span>
                      <span
                        v-else-if="diff.isRenamed"
                        class="text-[9px] uppercase font-bold px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-400 border border-purple-500/30 shrink-0"
                      >
                        Ren
                      </span>
                      <span
                        v-else
                        class="text-[9px] uppercase font-bold px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-400 border border-amber-500/30 shrink-0"
                      >
                        Mod
                      </span>

                      <span class="font-mono text-xs text-zinc-300 truncate">
                        {{ diff.newPath || diff.oldPath }}
                      </span>
                    </div>

                    <!-- Line Changes Stats -->
                    <div class="flex items-center gap-1.5 text-xs font-mono shrink-0">
                      <span class="text-emerald-400 font-medium">+{{ diff.additions }}</span>
                      <span class="text-red-400 font-medium">-{{ diff.deletions }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Load More Button -->
          <div v-if="hasMore && commits.length > 0" class="pt-2 text-center">
            <button
              type="button"
              :disabled="loadingMore"
              class="w-full py-2.5 px-4 rounded-xl text-xs font-medium bg-zinc-800/80 hover:bg-zinc-700/80 text-zinc-300 hover:text-white border border-zinc-700/60 transition-all flex items-center justify-center gap-2 disabled:opacity-50"
              @click="loadCommits(true)"
            >
              <Loader2 v-if="loadingMore" class="w-3.5 h-3.5 animate-spin text-purple-400" />
              <span>{{ loadingMore ? 'Loading more commits...' : 'Load more commits' }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
