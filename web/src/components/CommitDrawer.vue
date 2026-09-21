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
  (e: 'commit', payload: { message: string; amend: boolean }): void
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
const isSubmitting = ref(false)
const isGenerating = ref(false)
const copilotError = ref<string | null>(null)

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

async function handleGenerateMessage() {
  if (!props.repoId || props.stagedCount === 0 || isGenerating.value) return
  isGenerating.value = true
  copilotError.value = null
  try {
    const res = await api.generateCommitMessage(props.repoId, message.value.trim())
    if (res.message) {
      message.value = res.message
    }
  } catch (err: any) {
    copilotError.value = err.message || 'Failed to generate commit message'
  } finally {
    isGenerating.value = false
  }
}

async function handleSubmit() {
  if (!canCommit.value || isSubmitting.value) return
  isSubmitting.value = true
  try {
    emit('commit', {
      message: message.value.trim(),
      amend: amend.value,
    })
    message.value = ''
    amend.value = false
    savedDraft.value = ''
    isOpen.value = false
    emit('update:open', false)
  } finally {
    isSubmitting.value = false
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
          <GitCommit class="w-4 h-4" />
        </div>

        <div>
          <span class="text-xs font-semibold text-zinc-200">
            {{ stagedCount === 0 ? 'No files staged' : `${stagedCount} ${stagedCount === 1 ? 'file' : 'files'} staged` }}
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

      <div>
        <textarea
          v-model="message"
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
            class="rounded border-zinc-700 text-emerald-500 focus:ring-0 focus:outline-none bg-zinc-800 w-4 h-4 accent-emerald-500"
          />
          <span>Amend previous commit</span>
        </label>
        <div v-else></div>

        <!-- Commit Button -->
        <button
          type="button"
          :disabled="!canCommit || isSubmitting"
          class="px-5 py-2.5 rounded-xl text-sm font-semibold flex items-center gap-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-40 disabled:cursor-not-allowed text-white shadow-md active:scale-95 transition-all"
          @click="handleSubmit"
        >
          <Check class="w-4 h-4 stroke-[2.5]" />
          <span>{{ amend ? 'Amend Commit' : 'Commit' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
