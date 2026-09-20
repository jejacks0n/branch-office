<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { Branch } from '../types'
import { api } from '../api'
import {
  GitBranch,
  Search,
  Plus,
  Check,
  X,
  ArrowRight,
  Globe,
  Loader2,
  AlertCircle,
  Archive,
} from 'lucide-vue-next'

const props = defineProps<{
  show: boolean
  repoId: string
  currentBranch: string
  repoName: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'switched', branch: string): void
  (e: 'created', branch: string): void
}>()

const branches = ref<Branch[]>([])
const loading = ref(false)
const searchQuery = ref('')
const activeTab = ref<'local' | 'remote'>('local')

const showNewForm = ref(false)
const newBranchName = ref('')
const baseBranch = ref('')
const isSubmitting = ref(false)
const switchingBranch = ref<string | null>(null)
const modalError = ref<string | null>(null)

watch(
  () => props.show,
  async (newVal) => {
    if (newVal && props.repoId) {
      showNewForm.value = false
      newBranchName.value = ''
      searchQuery.value = ''
      modalError.value = null
      baseBranch.value = props.currentBranch || 'main'
      activeTab.value = 'local'
      await loadBranches()
    }
  }
)

async function loadBranches() {
  if (!props.repoId) return
  loading.value = true
  modalError.value = null
  try {
    branches.value = await api.getBranches(props.repoId)
  } catch (err: any) {
    modalError.value = err.message || 'Failed to fetch branches'
  } finally {
    loading.value = false
  }
}

const localBranches = computed(() => branches.value.filter((b) => !b.isRemote))
const remoteBranches = computed(() => branches.value.filter((b) => b.isRemote))

const filteredBranches = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  const list = activeTab.value === 'local' ? localBranches.value : remoteBranches.value
  if (!query) return list
  return list.filter(
    (b) =>
      b.name.toLowerCase().includes(query) ||
      (b.commitMsg && b.commitMsg.toLowerCase().includes(query)) ||
      (b.upstream && b.upstream.toLowerCase().includes(query))
  )
})

const failedBranch = ref<Branch | null>(null)
const isStashingAndSwitching = ref(false)

const canStashAndSwitch = computed(() => {
  if (!modalError.value || !failedBranch.value) return false
  const msg = modalError.value.toLowerCase()
  return msg.includes('overwritten by checkout') || msg.includes('stash them') || msg.includes('local changes')
})

async function handleSwitch(branch: Branch) {
  if (branch.isCurrent || switchingBranch.value || isSubmitting.value) return
  switchingBranch.value = branch.name
  modalError.value = null
  failedBranch.value = null
  try {
    const res = await api.checkoutBranch(props.repoId, branch.name)
    emit('switched', res.branch || branch.name)
    emit('close')
  } catch (err: any) {
    failedBranch.value = branch
    modalError.value = err.message || `Failed to switch to ${branch.name}`
  } finally {
    switchingBranch.value = null
  }
}

async function handleStashAndSwitch() {
  if (!failedBranch.value || isStashingAndSwitching.value) return
  isStashingAndSwitching.value = true
  try {
    await api.createStash(props.repoId, `WIP before switching to ${failedBranch.value.name}`, true)
    const res = await api.checkoutBranch(props.repoId, failedBranch.value.name)
    emit('switched', res.branch || failedBranch.value.name)
    emit('close')
  } catch (err: any) {
    modalError.value = err.message || `Failed to stash & switch to ${failedBranch.value.name}`
  } finally {
    isStashingAndSwitching.value = false
  }
}

async function handleCreate() {
  const name = newBranchName.value.trim()
  if (!name || isSubmitting.value) return
  isSubmitting.value = true
  modalError.value = null
  try {
    const res = await api.createBranch(props.repoId, name, baseBranch.value)
    emit('created', res.branch || name)
    emit('close')
  } catch (err: any) {
    modalError.value = err.message || `Failed to create branch ${name}`
  } finally {
    isSubmitting.value = false
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
      <div class="px-5 py-4 border-b border-zinc-800/80 flex items-center justify-between shrink-0">
        <div class="flex items-center gap-2.5">
          <div class="p-2 rounded-xl bg-emerald-500/10 text-emerald-400">
            <GitBranch class="w-5 h-5" />
          </div>
          <div>
            <h2 class="text-base font-semibold text-zinc-100">Git Branches</h2>
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

      <!-- Error Alert in Modal -->
      <div
        v-if="modalError"
        class="mx-4 mt-3 p-3 rounded-2xl bg-red-500/10 border border-red-500/20 text-red-400 text-xs flex flex-col gap-2 shrink-0"
      >
        <div class="flex items-start gap-2">
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

        <div v-if="canStashAndSwitch && failedBranch" class="pl-6 pt-1 flex items-center gap-2">
          <button
            type="button"
            :disabled="isStashingAndSwitching"
            class="px-3 py-1.5 rounded-xl bg-amber-600 hover:bg-amber-500 disabled:opacity-50 text-white font-semibold text-xs flex items-center gap-1.5 transition-all shadow-sm active:scale-95"
            @click="handleStashAndSwitch"
          >
            <Loader2 v-if="isStashingAndSwitching" class="w-3.5 h-3.5 animate-spin" />
            <Archive v-else class="w-3.5 h-3.5" />
            <span>Stash Changes & Switch to {{ failedBranch.name }}</span>
          </button>
        </div>
      </div>

      <!-- Action & Search Bar -->
      <div class="p-3.5 border-b border-zinc-800/80 bg-zinc-950/40 space-y-2.5 shrink-0">
        <div class="flex items-center gap-2">
          <!-- Search Input -->
          <div class="relative flex-1">
            <Search class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500 pointer-events-none" />
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Search branches..."
              class="w-full pl-9 pr-8 py-1.5 bg-zinc-900 border border-zinc-800 rounded-xl text-xs text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-emerald-500 font-mono"
            />
            <button
              v-if="searchQuery"
              type="button"
              class="absolute right-2.5 top-1/2 -translate-y-1/2 p-0.5 rounded text-zinc-500 hover:text-zinc-300"
              @click="searchQuery = ''"
            >
              <X class="w-3 h-3" />
            </button>
          </div>

          <!-- New Branch Toggle Button -->
          <button
            type="button"
            :class="[
              'px-3 py-1.5 rounded-xl border text-xs font-semibold flex items-center gap-1.5 transition-all active:scale-95 shrink-0',
              showNewForm
                ? 'bg-zinc-800 text-zinc-300 border-zinc-700'
                : 'bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-400 border-emerald-500/30',
            ]"
            @click="showNewForm = !showNewForm"
          >
            <Plus v-if="!showNewForm" class="w-3.5 h-3.5" />
            <X v-else class="w-3.5 h-3.5" />
            <span>{{ showNewForm ? 'Cancel' : 'New Branch' }}</span>
          </button>
        </div>

        <!-- Local vs Remote Tab Navigation -->
        <div class="flex items-center gap-1.5">
          <button
            type="button"
            :class="[
              'px-3 py-1 rounded-lg text-xs font-semibold transition-colors flex items-center gap-1.5 border',
              activeTab === 'local'
                ? 'bg-zinc-800 text-zinc-100 border-zinc-700'
                : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/40 border-transparent',
            ]"
            @click="activeTab = 'local'"
          >
            <GitBranch class="w-3 h-3" />
            <span>Local ({{ localBranches.length }})</span>
          </button>
          <button
            v-if="remoteBranches.length > 0"
            type="button"
            :class="[
              'px-3 py-1 rounded-lg text-xs font-semibold transition-colors flex items-center gap-1.5 border',
              activeTab === 'remote'
                ? 'bg-zinc-800 text-zinc-100 border-zinc-700'
                : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/40 border-transparent',
            ]"
            @click="activeTab = 'remote'"
          >
            <Globe class="w-3 h-3" />
            <span>Remote ({{ remoteBranches.length }})</span>
          </button>
        </div>
      </div>

      <!-- New Branch Form -->
      <div v-if="showNewForm" class="p-4 border-b border-zinc-800 bg-zinc-950/80 space-y-3 shrink-0 animate-fade-in">
        <h3 class="text-xs font-semibold text-zinc-200 uppercase tracking-wider">Create New Branch</h3>

        <div class="space-y-1">
          <label class="text-xs text-zinc-400">Branch Name</label>
          <input
            v-model="newBranchName"
            type="text"
            placeholder="e.g. feature-login"
            class="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-xl text-xs text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-emerald-500 font-mono"
            @keyup.enter="handleCreate"
          />
        </div>

        <div class="space-y-1">
          <label class="text-xs text-zinc-400">Based on</label>
          <select
            v-model="baseBranch"
            class="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-xl text-xs text-zinc-100 focus:outline-none focus:border-emerald-500 font-mono"
          >
            <option v-for="b in localBranches" :key="b.name" :value="b.name">
              {{ b.name }} {{ b.isCurrent ? '(current)' : '' }}
            </option>
          </select>
        </div>

        <div class="pt-2 flex items-center justify-between">
          <p class="text-[11px] text-zinc-500 font-mono">
            Immediately checks out after creation.
          </p>
          <button
            type="button"
            :disabled="!newBranchName.trim() || isSubmitting"
            class="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-40 text-white rounded-xl text-xs font-semibold flex items-center gap-1.5 transition-all shadow-md active:scale-95"
            @click="handleCreate"
          >
            <Loader2 v-if="isSubmitting" class="w-3.5 h-3.5 animate-spin" />
            <Plus v-else class="w-3.5 h-3.5" />
            <span>Create & Switch</span>
          </button>
        </div>
      </div>

      <!-- Branches List -->
      <div class="flex-1 overflow-y-auto p-4 space-y-2 min-h-[160px]">
        <div v-if="loading" class="py-12 text-center text-zinc-500 space-y-2">
          <Loader2 class="w-6 h-6 animate-spin mx-auto text-emerald-400" />
          <p class="text-xs">Loading branches...</p>
        </div>

        <div v-else-if="filteredBranches.length === 0" class="py-12 text-center text-zinc-500 space-y-2">
          <GitBranch class="w-8 h-8 mx-auto text-zinc-700" />
          <p class="text-xs">No {{ activeTab }} branches found.</p>
        </div>

        <div
          v-for="b in filteredBranches"
          :key="b.name"
          :class="[
            'p-3 rounded-2xl border transition-all flex items-center justify-between gap-3 group min-h-[66px]',
            b.isCurrent
              ? 'bg-emerald-950/20 border-emerald-500/40 cursor-default'
              : 'bg-zinc-900/60 border-zinc-800/80 hover:bg-zinc-800/60 hover:border-zinc-700 cursor-pointer active:scale-[0.99]',
          ]"
          @click="!b.isCurrent && handleSwitch(b)"
        >
          <!-- Branch Info -->
          <div class="min-w-0 flex-1">
            <div class="h-5 flex items-center gap-2">
              <component
                :is="b.isRemote ? Globe : GitBranch"
                :class="['w-4 h-4 shrink-0', b.isCurrent ? 'text-emerald-400' : 'text-zinc-400 group-hover:text-zinc-300']"
              />
              <span class="font-mono text-xs font-semibold text-zinc-100 truncate">
                {{ b.name }}
              </span>

              <span
                v-if="b.isCurrent"
                class="h-5 inline-flex items-center text-[10px] font-mono leading-none px-1.5 rounded bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 font-semibold shrink-0"
              >
                current
              </span>

              <span
                v-if="b.upstream"
                class="h-5 inline-flex items-center text-[10px] font-mono leading-none px-1.5 rounded bg-zinc-800 text-zinc-400 border border-zinc-700/60 shrink-0 truncate max-w-[130px]"
                :title="b.upstream"
              >
                ↑ {{ b.upstream }}
              </span>
            </div>

            <!-- Last Commit info if available -->
            <div class="mt-1 h-4 flex items-center gap-2 text-[11px] text-zinc-500 font-mono">
              <template v-if="b.commitMsg || b.commitHash">
                <span v-if="b.commitHash" class="text-zinc-400 shrink-0">{{ b.commitHash }}</span>
                <span v-if="b.commitMsg" class="truncate text-zinc-500">{{ b.commitMsg }}</span>
              </template>
              <span v-else class="text-zinc-600 italic">No commits</span>
            </div>
          </div>

          <!-- Switch Action Button / Indicator -->
          <div class="shrink-0 flex items-center">
            <div
              v-if="b.isCurrent"
              class="w-7 h-7 rounded-xl bg-emerald-500/10 text-emerald-400 flex items-center justify-center border border-emerald-500/20"
              title="Current active branch"
            >
              <Check class="w-3.5 h-3.5" />
            </div>

            <button
              v-else
              type="button"
              :disabled="switchingBranch !== null"
              class="h-7 px-3 rounded-xl bg-zinc-800 hover:bg-emerald-600 hover:text-white text-zinc-300 border border-zinc-700/80 text-xs font-semibold flex items-center gap-1.5 transition-all active:scale-95"
              @click.stop="handleSwitch(b)"
            >
              <Loader2 v-if="switchingBranch === b.name" class="w-3.5 h-3.5 animate-spin text-emerald-400" />
              <template v-else>
                <span>Switch</span>
                <ArrowRight class="w-3 h-3 text-zinc-400 group-hover:text-white" />
              </template>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
  </Teleport>
</template>
