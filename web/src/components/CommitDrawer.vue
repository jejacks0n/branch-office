<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { GitCommit, ChevronUp, ChevronDown, Check, Sparkles, AlertCircle } from 'lucide-vue-next'
import { api } from '../api'
import ThinkingOrb from './ThinkingOrb.vue'

const props = defineProps<{
  stagedCount: number
  hasHead: boolean
  repoId?: string
  lastCommitMessage?: string
  open?: boolean
}>()

const emit = defineEmits<{
  (e: 'committed'): void
  (e: 'update:open', open: boolean): void
}>()

const isOpen = ref(props.open ?? false)

watch(
  () => props.open,
  (val) => {
    if (val !== undefined && val !== isOpen.value) {
      isOpen.value = val
    }
  }
)
const message = ref('')
const amend = ref(false)
const savedDraft = ref('')
// Which repo a commit or Copilot request is running for, so switching repos
// mid-request doesn't show its progress, or land its result, in another repo.
const submittingRepo = ref<string | null>(null)
const generatingRepo = ref<string | null>(null)
const isSubmitting = computed(() => !!props.repoId && submittingRepo.value === props.repoId)
const isGenerating = computed(() => !!props.repoId && generatingRepo.value === props.repoId)
const copilotError = ref<string | null>(null)
const commitError = ref<string | null>(null)

// Unsent messages, per repo, kept in localStorage so they survive reloads
// (phones often reload a backgrounded web app). The drawer outlives repo
// switches, so each repo's draft is swapped in when it becomes active.
const DRAFTS_KEY = 'broffice_commit_drafts'

function loadDrafts(): Record<string, string> {
  try {
    return JSON.parse(localStorage.getItem(DRAFTS_KEY) || '{}') || {}
  } catch {
    return {}
  }
}

const drafts = loadDrafts()

function setDraft(repoId: string, text: string) {
  if (text.trim()) drafts[repoId] = text
  else delete drafts[repoId]
  try {
    localStorage.setItem(DRAFTS_KEY, JSON.stringify(drafts))
  } catch {
    // Storage unavailable (private mode); drafts still last for this session.
  }
}

watch(
  () => props.repoId,
  (next) => {
    amend.value = false
    savedDraft.value = ''
    message.value = (next && drafts[next]) || ''
    commitError.value = null
    copilotError.value = null
  },
  { immediate: true }
)

// Save on every edit. While amending, the box holds the last commit's
// message; the user's own draft is in savedDraft.
watch([message, amend, savedDraft], () => {
  if (props.repoId) setDraft(props.repoId, amend.value ? savedDraft.value : message.value)
})

watch(amend, async (isAmending) => {
  if (isAmending) {
    if (message.value.trim() && message.value !== props.lastCommitMessage) {
      savedDraft.value = message.value
    }
    if (props.lastCommitMessage) {
      message.value = props.lastCommitMessage
    } else if (props.repoId && props.hasHead) {
      try {
        const res = await api.getLastCommitMessage(props.repoId)
        if (res.message) {
          message.value = res.message
        }
      } catch (err: any) {
        console.error('Failed to get last commit message', err)
      }
    }
  } else {
    if (message.value === props.lastCommitMessage || !message.value.trim()) {
      message.value = savedDraft.value
      savedDraft.value = ''
    }
  }
})

watch(
  () => props.lastCommitMessage,
  (newMsg) => {
    if (amend.value && newMsg && (!message.value || message.value === savedDraft.value)) {
      message.value = newMsg
    }
  }
)

const canCommit = computed(() => {
  return message.value.trim().length > 0 && (props.stagedCount > 0 || amend.value)
})

// Copilot's last output per repo. The box is sent as author guidance, but not
// when it still holds that output: guiding Copilot with its own message just
// gets the same message back, so an unedited box asks for a fresh one.
const lastGenerated = new Map<string, string>()

async function handleGenerateMessage() {
  const repoId = props.repoId
  if (!repoId || props.stagedCount === 0 || isGenerating.value) return
  generatingRepo.value = repoId
  copilotError.value = null
  const current = message.value.trim()
  const hint = current === lastGenerated.get(repoId) ? '' : current
  try {
    const res = await api.generateCommitMessage(repoId, hint)
    if (res.message) {
      lastGenerated.set(repoId, res.message.trim())
      if (props.repoId === repoId) message.value = res.message
      else setDraft(repoId, res.message)
    }
  } catch (err: any) {
    if (props.repoId === repoId) copilotError.value = err.message || 'Failed to generate commit message'
  } finally {
    if (generatingRepo.value === repoId) generatingRepo.value = null
  }
}

// Commits can take a while (hooks, signing), so the drawer waits for the result:
// it shows progress meanwhile and keeps the message if the commit fails.
async function handleSubmit() {
  const repoId = props.repoId
  if (!repoId || !canCommit.value || isSubmitting.value) return
  submittingRepo.value = repoId
  commitError.value = null
  try {
    await api.commit(repoId, message.value.trim(), amend.value)
    setDraft(repoId, '')
    if (props.repoId === repoId) {
      message.value = ''
      amend.value = false
      savedDraft.value = ''
      isOpen.value = false
      emit('update:open', false)
    }
    emit('committed')
  } catch (err: any) {
    // After a switch away the draft is already parked for that repo; its
    // staged files still show the commit didn't go through.
    if (props.repoId === repoId) {
      commitError.value = err.message || 'Commit failed'
      isOpen.value = true
      emit('update:open', true)
    }
  } finally {
    if (submittingRepo.value === repoId) submittingRepo.value = null
  }
}

function toggleOpen() {
  isOpen.value = !isOpen.value
  emit('update:open', isOpen.value)
}
</script>

<template>
  <div
    class="fixed inset-x-0 bottom-0 z-40 bg-zinc-900/95 backdrop-blur-md border-t border-zinc-800 shadow-2xl transition-all duration-300"
  >
    <!-- Collapsed summary bar (always visible at bottom) -->
    <div
      :class="[
        'px-4 pt-3 flex items-center justify-between cursor-pointer active:bg-zinc-800/40 select-none',
        isOpen ? 'pb-3' : 'pb-safe',
      ]"
      @click="toggleOpen"
    >
      <div class="flex items-center gap-2.5">
        <div
          :class="[
            'p-2 rounded-xl border transition-colors',
            stagedCount > 0
              ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
              : 'bg-zinc-800 border-zinc-700/60 text-zinc-400',
          ]"
        >
          <ThinkingOrb v-if="isSubmitting" state="working" :size="16" />
          <GitCommit v-else class="w-4 h-4" />
        </div>

        <div>
          <span class="text-xs font-semibold text-zinc-200">
            <template v-if="isSubmitting">{{ amend ? 'Amending commit…' : 'Committing…' }}</template>
            <template v-else>
              {{ stagedCount === 0 ? 'No files staged' : `${stagedCount} ${stagedCount === 1 ? 'file' : 'files'} staged` }}
            </template>
          </span>
          <span v-if="amend" class="ml-2 text-[10px] font-mono px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20">
            Amend
          </span>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <span class="text-xs text-zinc-400 font-medium">
          {{ isOpen ? 'Close' : 'Draft Commit' }}
        </span>
        <component :is="isOpen ? ChevronDown : ChevronUp" class="w-4 h-4 text-zinc-400" />
      </div>
    </div>

    <!-- Expanded Drawer Content -->
    <div v-show="isOpen" class="px-4 pb-safe-lg space-y-2.5 pt-1.5 border-t border-zinc-800/40">
      <!-- Toolbar row above message input -->
      <div class="flex items-center justify-between">
        <span class="text-xs text-zinc-400 font-medium">Commit Message</span>
        <button
          type="button"
          :disabled="stagedCount === 0 || isGenerating || !repoId"
          class="flex items-center gap-1.5 px-2.5 py-1 rounded-xl bg-purple-500/10 hover:bg-purple-500/20 text-purple-300 border border-purple-500/30 disabled:opacity-40 disabled:cursor-not-allowed transition-all active:scale-95 text-xs font-medium"
          title="Generate commit message from staged diff using GitHub Copilot"
          @click="handleGenerateMessage"
        >
          <ThinkingOrb v-if="isGenerating" state="working" :size="20" class="shrink-0" />
          <Sparkles v-else class="w-5 h-5 text-purple-400 shrink-0" />
          <span>{{ isGenerating ? 'Thinking...' : 'Copilot' }}</span>
        </button>
      </div>

      <!-- Copilot Error Alert -->
      <div
        v-if="copilotError"
        class="p-2.5 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-xs flex items-center justify-between gap-2"
      >
        <div class="flex items-center gap-1.5 min-w-0">
          <AlertCircle class="w-3.5 h-3.5 shrink-0" />
          <span class="truncate font-mono text-[11px]">{{ copilotError }}</span>
        </div>
        <button class="text-red-400 hover:text-red-200 shrink-0 font-bold text-xs" @click="copilotError = null">
          ✕
        </button>
      </div>

      <!-- Commit Error Alert: hook output can span many lines -->
      <div
        v-if="commitError"
        class="p-2.5 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-xs flex items-start justify-between gap-2"
      >
        <div class="flex items-start gap-1.5 min-w-0">
          <AlertCircle class="w-3.5 h-3.5 shrink-0 mt-px" />
          <pre class="font-mono text-[11px] whitespace-pre-wrap break-words max-h-32 overflow-y-auto select-text">{{ commitError }}</pre>
        </div>
        <button class="text-red-400 hover:text-red-200 shrink-0 font-bold text-xs" @click="commitError = null">
          ✕
        </button>
      </div>

      <div>
        <textarea
          v-model="message"
          :readonly="isSubmitting"
          rows="5"
          placeholder="Commit message (or tap Copilot to generate)..."
          class="w-full p-3 bg-zinc-950/80 border border-zinc-700/80 rounded-2xl text-sm text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-emerald-500 font-sans transition-colors resize-none min-h-[130px] sm:min-h-[150px]"
        ></textarea>
      </div>

      <!-- Controls row -->
      <div class="flex items-center justify-between gap-3">
        <!-- Amend toggle -->
        <label
          v-if="hasHead"
          class="flex items-center gap-2 cursor-pointer text-xs font-medium text-zinc-400 hover:text-zinc-200 select-none"
        >
          <input
            type="checkbox"
            v-model="amend"
            :disabled="isSubmitting"
            class="rounded border-zinc-700 text-emerald-500 focus:ring-0 focus:outline-none bg-zinc-800 w-4 h-4 accent-emerald-500"
          />
          <span>Amend previous commit</span>
        </label>
        <div v-else></div>

        <!-- Commit Button -->
        <button
          type="button"
          :disabled="!canCommit || isSubmitting"
          :aria-busy="isSubmitting"
          class="px-5 py-2.5 rounded-xl text-sm font-semibold flex items-center gap-2 bg-emerald-600 hover:bg-emerald-500 disabled:cursor-not-allowed aria-busy:!opacity-100 disabled:opacity-40 text-white shadow-md active:scale-95 transition-all"
          @click="handleSubmit"
        >
          <ThinkingOrb v-if="isSubmitting" state="working" :size="16" class="shrink-0" />
          <Check v-else class="w-4 h-4 stroke-[2.5]" />
          <span>{{ isSubmitting ? (amend ? 'Amending…' : 'Committing…') : amend ? 'Amend Commit' : 'Commit' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
