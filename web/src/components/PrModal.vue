<script setup lang="ts">
import { ref, watch } from 'vue'
import type { PRStatus } from '../types'
import { GitPullRequest, ExternalLink, X, Plus, AlertCircle, CheckCircle2 } from 'lucide-vue-next'

const props = defineProps<{
  show: boolean
  status: PRStatus | null
  branch: string
  lastCommitMessage?: string
}>()

const emit = defineEmits<{
  (e: 'create-pr', data: { title: string; body: string; draft: boolean }): void
  (e: 'close'): void
}>()

const title = ref('')
const body = ref('')
const draft = ref(false)
const isSubmitting = ref(false)

watch(
  () => props.show,
  (newVal) => {
    if (newVal) {
      if (!title.value && props.lastCommitMessage) {
        title.value = props.lastCommitMessage
      } else if (!title.value) {
        title.value = props.branch.replace(/[-_]/g, ' ')
      }
    }
  }
)

async function handleSubmit() {
  if (!title.value.trim() || isSubmitting.value) return
  isSubmitting.value = true
  try {
    emit('create-pr', {
      title: title.value.trim(),
      body: body.value.trim(),
      draft: draft.value,
    })
  } finally {
    isSubmitting.value = false
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
        class="modal-panel max-w-lg bg-zinc-900 border border-zinc-800 rounded-3xl shadow-2xl p-5 space-y-5 flex flex-col overflow-y-auto"
      >
      <!-- Header -->
      <div class="flex items-center justify-between pb-3 border-b border-zinc-800 shrink-0">
        <div class="flex items-center gap-2.5">
          <div class="p-2 rounded-xl bg-zinc-800 text-purple-400">
            <GitPullRequest class="w-5 h-5" />
          </div>
          <div>
            <h3 class="text-base font-semibold text-zinc-100">GitHub Pull Request</h3>
            <p class="text-xs text-zinc-400 font-mono">{{ branch }}</p>
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

      <!-- Content -->
      <div class="flex-1 overflow-y-auto space-y-4">
        <!-- GH CLI Not Installed Notice -->
        <div
          v-if="status && !status.installed"
          class="p-4 rounded-2xl bg-amber-500/10 border border-amber-500/20 text-amber-400 text-xs flex items-start gap-2.5"
        >
          <AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
          <div>
            <span class="font-semibold">GitHub CLI (`gh`) not detected</span>
            <p class="text-zinc-400 mt-1">
              Install and authenticate the GitHub CLI on your host machine to view and create pull requests.
            </p>
          </div>
        </div>

        <!-- PR Already Exists -->
        <div
          v-else-if="status && status.exists && status.pr"
          class="p-4 rounded-2xl bg-zinc-950/60 border border-zinc-800 space-y-3"
        >
          <div class="flex items-center justify-between">
            <span
              :class="[
                'text-xs font-bold font-mono px-2 py-0.5 rounded-full border',
                status.pr.state === 'OPEN'
                  ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                  : status.pr.state === 'MERGED'
                  ? 'bg-purple-500/10 text-purple-400 border-purple-500/20'
                  : 'bg-zinc-800 text-zinc-400 border-zinc-700',
              ]"
            >
              {{ status.pr.state }}
            </span>
            <span class="text-xs text-zinc-400 font-mono">#{{ status.pr.number }}</span>
          </div>

          <h4 class="text-sm font-semibold text-zinc-100">{{ status.pr.title }}</h4>

          <div v-if="status.pr.body" class="p-3 bg-zinc-900 rounded-xl text-xs text-zinc-400 font-mono whitespace-pre-wrap max-h-32 overflow-y-auto">
            {{ status.pr.body }}
          </div>

          <div class="pt-2">
            <a
              :href="status.pr.url"
              target="_blank"
              rel="noopener noreferrer"
              class="w-full py-2.5 px-4 rounded-xl text-xs font-semibold bg-purple-600 hover:bg-purple-500 text-white flex items-center justify-center gap-2 transition-all active:scale-[0.99]"
            >
              <span>View PR on GitHub</span>
              <ExternalLink class="w-3.5 h-3.5" />
            </a>
          </div>
        </div>

        <!-- Create New PR Form -->
        <div v-else class="space-y-3.5">
          <div class="space-y-1.5">
            <label class="text-xs font-medium text-zinc-300">Title</label>
            <input
              v-model="title"
              type="text"
              placeholder="e.g. feat: add mobile git control"
              class="w-full px-3.5 py-2.5 bg-zinc-950 border border-zinc-700 rounded-xl text-sm text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-purple-500"
            />
          </div>

          <div class="space-y-1.5">
            <label class="text-xs font-medium text-zinc-300">Description</label>
            <textarea
              v-model="body"
              rows="4"
              placeholder="Summary of changes (Markdown supported)..."
              class="w-full p-3 bg-zinc-950 border border-zinc-700 rounded-xl text-sm text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-purple-500 resize-none font-sans"
            ></textarea>
          </div>

          <div class="flex items-center gap-2 text-xs text-zinc-300">
            <input
              type="checkbox"
              id="draftpr"
              v-model="draft"
              class="rounded border-zinc-700 text-purple-500 bg-zinc-800 w-4 h-4 accent-purple-500"
            />
            <label for="draftpr" class="cursor-pointer">Create as Draft Pull Request</label>
          </div>

          <div class="pt-2">
            <button
              type="button"
              :disabled="!title.trim() || isSubmitting"
              class="w-full py-3 px-4 rounded-xl text-sm font-semibold bg-purple-600 hover:bg-purple-500 disabled:opacity-50 text-white flex items-center justify-center gap-2 transition-all shadow-md active:scale-[0.99]"
              @click="handleSubmit"
            >
              <GitPullRequest class="w-4 h-4" />
              <span>{{ draft ? 'Create Draft PR' : 'Open Pull Request' }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
  </Teleport>
</template>
