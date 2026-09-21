<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import {
  Tag,
  UploadCloud,
  Plus,
  X,
  Trash2,
  Loader2,
  AlertCircle,
  Check,
  Search,
  GitCommit,
  Clock,
  ExternalLink,
} from 'lucide-vue-next'
import type { TagItem } from '../types'
import { api } from '../api'

const props = defineProps<{
  show: boolean
  repoId: string
  repoName: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'changed'): void
}>()

const tags = ref<TagItem[]>([])
const loading = ref(false)
const isSubmitting = ref(false)
const pushingTag = ref<string | null>(null)
const modalError = ref<string | null>(null)
const successMsg = ref<string | null>(null)

// Search query
const searchQuery = ref('')

// Tag create form
const showNewForm = ref(false)
const tagName = ref('')
const tagMessage = ref('')
const pushImmediately = ref(true)

// Tag delete confirmation
const tagToDelete = ref<TagItem | null>(null)
const deleteRemoteAlso = ref(false)
const isDeleting = ref(false)

const isValidTagName = computed(() => tagName.value.trim().length > 0)

const filteredTags = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return tags.value
  return tags.value.filter(
    (t) =>
      t.name.toLowerCase().includes(q) ||
      t.commitHash.toLowerCase().includes(q) ||
      t.message.toLowerCase().includes(q)
  )
})

watch(
  [() => props.show, () => props.repoId],
  async ([show, repoId]) => {
    if (show && repoId) {
      modalError.value = null
      successMsg.value = null
      searchQuery.value = ''
      showNewForm.value = false
      tagName.value = ''
      tagMessage.value = ''
      pushImmediately.value = true
      await loadTags()
      if (tags.value.length === 0) {
        showNewForm.value = true
      }
    } else {
      tags.value = []
      showNewForm.value = false
      modalError.value = null
      successMsg.value = null
    }
  }
)

async function loadTags() {
  if (!props.repoId) {
    tags.value = []
    return
  }
  loading.value = true
  try {
    const res = await api.getTags(props.repoId)
    tags.value = Array.isArray(res) ? res : []
  } catch (err: any) {
    tags.value = []
    modalError.value = err.message || 'Failed to load tags'
  } finally {
    loading.value = false
  }
}

async function handleCreate() {
  if (!props.repoId || isSubmitting.value || !isValidTagName.value) return
  isSubmitting.value = true
  modalError.value = null
  successMsg.value = null
  try {
    const name = tagName.value.trim()
    await api.createTag(props.repoId, {
      name,
      message: tagMessage.value.trim(),
      push: pushImmediately.value,
    })

    successMsg.value = pushImmediately.value
      ? `Tag ${name} created and pushed to origin!`
      : `Tag ${name} created successfully!`

    tagName.value = ''
    tagMessage.value = ''
    showNewForm.value = false
    await loadTags()
    emit('changed')

    setTimeout(() => {
      if (successMsg.value) successMsg.value = null
    }, 4000)
  } catch (err: any) {
    modalError.value = err.message || 'Failed to create tag'
  } finally {
    isSubmitting.value = false
  }
}

async function handlePushTag(tag: TagItem) {
  if (!props.repoId || pushingTag.value) return
  pushingTag.value = tag.name
  modalError.value = null
  successMsg.value = null
  try {
    await api.pushTag(props.repoId, tag.name)
    successMsg.value = `Pushed tag ${tag.name} to origin!`
    setTimeout(() => {
      if (successMsg.value) successMsg.value = null
    }, 4000)
  } catch (err: any) {
    modalError.value = err.message || `Failed to push tag ${tag.name}`
  } finally {
    pushingTag.value = null
  }
}

function promptDeleteTag(tag: TagItem) {
  tagToDelete.value = tag
  deleteRemoteAlso.value = false
}

async function confirmDeleteTag() {
  if (!props.repoId || !tagToDelete.value || isDeleting.value) return
  isDeleting.value = true
  modalError.value = null
  successMsg.value = null
  const name = tagToDelete.value.name
  try {
    await api.deleteTag(props.repoId, name, deleteRemoteAlso.value)
    successMsg.value = deleteRemoteAlso.value
      ? `Deleted tag ${name} locally and from origin.`
      : `Deleted local tag ${name}.`

    tagToDelete.value = null
    deleteRemoteAlso.value = false
    await loadTags()
    emit('changed')

    setTimeout(() => {
      if (successMsg.value) successMsg.value = null
    }, 4000)
  } catch (err: any) {
    modalError.value = err.message || `Failed to delete tag ${name}`
  } finally {
    isDeleting.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="show"
      class="viewport-fixed modal-overlay z-50 bg-black/80 backdrop-blur-sm animate-fade-in"
      @click.self="emit('close')"
    >
      <div
        class="modal-panel max-w-lg bg-zinc-900 border border-zinc-800 rounded-3xl shadow-2xl flex flex-col overflow-hidden transform transition-all"
      >
        <!-- Modal Header -->
        <div class="px-5 py-4 border-b border-zinc-800/80 flex items-center justify-between shrink-0 bg-zinc-900/60">
          <div class="flex items-center gap-3">
            <div class="p-2.5 rounded-2xl bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
              <Tag class="w-5 h-5" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h2 class="text-base font-semibold text-zinc-100">Git Tags</h2>
                <span
                  v-if="tags.length > 0"
                  class="px-2 py-0.5 rounded-full text-[11px] font-mono font-bold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30"
                >
                  {{ tags.length }}
                </span>
              </div>
              <p class="text-xs text-zinc-400 font-mono truncate max-w-[220px] sm:max-w-xs">
                {{ repoName }}
              </p>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <button
              type="button"
              :class="[
                'flex items-center gap-1 px-3 py-1.5 rounded-xl text-xs font-semibold transition-all active:scale-95',
                showNewForm
                  ? 'bg-zinc-800 text-zinc-300 border border-zinc-700'
                  : 'bg-emerald-600 hover:bg-emerald-500 text-white shadow-sm shadow-emerald-950/40',
              ]"
              @click="showNewForm = !showNewForm"
            >
              <Plus :class="['w-3.5 h-3.5 transition-transform', showNewForm ? 'rotate-45' : '']" />
              <span>{{ showNewForm ? 'Cancel' : 'New Tag' }}</span>
            </button>

            <button
              type="button"
              class="p-2 rounded-xl text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/60 transition-colors"
              @click="emit('close')"
            >
              <X class="w-5 h-5" />
            </button>
          </div>
        </div>

        <!-- Alert messages -->
        <div v-if="modalError" class="mx-4 mt-3 p-3 rounded-2xl bg-red-500/10 border border-red-500/20 text-red-400 text-xs flex items-center justify-between gap-2 shrink-0">
          <div class="flex items-center gap-2">
            <AlertCircle class="w-4 h-4 shrink-0" />
            <span>{{ modalError }}</span>
          </div>
          <button type="button" class="p-1 hover:text-red-300" @click="modalError = null">
            <X class="w-3.5 h-3.5" />
          </button>
        </div>

        <div v-if="successMsg" class="mx-4 mt-3 p-3 rounded-2xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs flex items-center justify-between gap-2 shrink-0">
          <div class="flex items-center gap-2">
            <Check class="w-4 h-4 shrink-0" />
            <span>{{ successMsg }}</span>
          </div>
          <button type="button" class="p-1 hover:text-emerald-300" @click="successMsg = null">
            <X class="w-3.5 h-3.5" />
          </button>
        </div>

        <!-- Create Tag Form (collapsible) -->
        <div
          v-if="showNewForm"
          class="m-4 p-4 rounded-2xl bg-zinc-950/60 border border-zinc-800/90 space-y-3 shrink-0"
        >
          <div class="flex items-center justify-between">
            <h3 class="text-xs font-semibold text-zinc-300 uppercase tracking-wider">Create New Tag</h3>
          </div>

          <div class="space-y-2.5">
            <div>
              <label class="block text-xs font-medium text-zinc-400 mb-1">Tag Name</label>
              <input
                v-model="tagName"
                type="text"
                placeholder="e.g. v0.1.0 or v1.0.0"
                class="w-full px-3.5 py-2.5 bg-zinc-900 border border-zinc-700/80 focus:border-emerald-500/80 rounded-xl text-xs text-zinc-200 placeholder-zinc-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 font-mono transition-colors"
                @keydown.enter="handleCreate"
              />
            </div>

            <div>
              <label class="block text-xs font-medium text-zinc-400 mb-1">
                Release Message <span class="text-zinc-500 font-normal">(optional, creates annotated tag)</span>
              </label>
              <textarea
                v-model="tagMessage"
                rows="2"
                placeholder="Release notes, version summary..."
                class="w-full px-3.5 py-2 bg-zinc-900 border border-zinc-700/80 focus:border-emerald-500/80 rounded-xl text-xs text-zinc-200 placeholder-zinc-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition-colors resize-none"
              />
            </div>

            <!-- Push to remote checkbox -->
            <label class="flex items-center gap-2 text-xs text-zinc-300 cursor-pointer select-none py-0.5">
              <input
                v-model="pushImmediately"
                type="checkbox"
                class="w-4 h-4 rounded border-zinc-700 bg-zinc-900 text-emerald-500 focus:ring-0 focus:ring-offset-0 transition-colors"
              />
              <span class="flex items-center gap-1.5">
                <span>Push to remote (<code class="text-emerald-400 font-mono">origin</code>) immediately</span>
                <UploadCloud class="w-3.5 h-3.5 text-sky-400" />
              </span>
            </label>

            <div class="flex items-center justify-end gap-2 pt-1">
              <button
                type="button"
                class="px-3.5 py-2 text-xs font-medium text-zinc-400 hover:text-zinc-200 rounded-xl transition-colors"
                @click="showNewForm = false"
              >
                Cancel
              </button>
              <button
                type="button"
                :disabled="!isValidTagName || isSubmitting"
                class="flex items-center gap-1.5 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-xl text-xs font-semibold shadow-sm transition-all active:scale-95"
                @click="handleCreate"
              >
                <Loader2 v-if="isSubmitting" class="w-3.5 h-3.5 animate-spin" />
                <UploadCloud v-else-if="pushImmediately" class="w-3.5 h-3.5" />
                <Tag v-else class="w-3.5 h-3.5" />
                <span>{{ isSubmitting ? 'Creating...' : pushImmediately ? 'Create & Push Tag' : 'Create Tag' }}</span>
              </button>
            </div>
          </div>
        </div>

        <!-- Search Bar (if tags exist) -->
        <div v-if="tags.length > 3" class="px-4 pb-2 shrink-0">
          <div class="relative">
            <Search class="w-4 h-4 text-zinc-500 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Search tags..."
              class="w-full pl-9 pr-3.5 py-2 bg-zinc-950/70 border border-zinc-800 rounded-xl text-xs text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-zinc-700 transition-colors"
            />
          </div>
        </div>

        <!-- Tags List Area -->
        <div class="flex-1 overflow-y-auto px-4 pb-5 space-y-2.5 overscroll-contain">
          <!-- Loading state -->
          <div v-if="loading && tags.length === 0" class="py-12 flex flex-col items-center justify-center gap-2 text-zinc-500">
            <Loader2 class="w-6 h-6 animate-spin text-emerald-400" />
            <span class="text-xs">Loading tags...</span>
          </div>

          <!-- Empty state -->
          <div
            v-else-if="tags.length === 0"
            class="py-12 px-4 flex flex-col items-center justify-center text-center gap-3 bg-zinc-950/30 rounded-2xl border border-zinc-800/40"
          >
            <div class="p-3 rounded-2xl bg-zinc-800/50 text-zinc-400">
              <Tag class="w-7 h-7" />
            </div>
            <div class="space-y-1 max-w-xs">
              <p class="text-sm font-medium text-zinc-300">No tags found</p>
              <p class="text-xs text-zinc-500 leading-relaxed">
                Create your first version tag above to mark release checkpoints.
              </p>
            </div>
          </div>

          <!-- Filtered Empty state -->
          <div
            v-else-if="filteredTags.length === 0"
            class="py-8 text-center text-xs text-zinc-500"
          >
            No tags matching "{{ searchQuery }}"
          </div>

          <!-- Tag Cards -->
          <div
            v-for="tag in filteredTags"
            :key="tag.name"
            class="p-3.5 rounded-2xl bg-zinc-950/50 border border-zinc-800/80 hover:border-zinc-700/80 transition-all space-y-2.5"
          >
            <!-- Top row: Name, Commit hash, Type badge, Relative Date -->
            <div class="flex items-start justify-between gap-2">
              <div class="flex flex-wrap items-center gap-2">
                <div class="flex items-center gap-1.5 px-2.5 py-1 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 font-mono font-bold text-xs">
                  <Tag class="w-3.5 h-3.5 shrink-0" />
                  <span>{{ tag.name }}</span>
                </div>

                <div class="flex items-center gap-1 px-2 py-0.5 rounded-lg bg-zinc-800/80 border border-zinc-700/60 text-zinc-400 font-mono text-[11px]" title="Commit Hash">
                  <GitCommit class="w-3 h-3 text-zinc-500" />
                  <span>{{ tag.commitHash }}</span>
                </div>

                <span
                  :class="[
                    'px-2 py-0.5 rounded-md text-[10px] font-semibold uppercase tracking-wider',
                    tag.isAnnotated
                      ? 'bg-purple-500/10 text-purple-400 border border-purple-500/20'
                      : 'bg-zinc-800 text-zinc-400 border border-zinc-700/50',
                  ]"
                >
                  {{ tag.isAnnotated ? 'Annotated' : 'Lightweight' }}
                </span>
              </div>

              <div class="flex items-center gap-1 text-[11px] text-zinc-500 shrink-0 font-mono">
                <Clock class="w-3 h-3" />
                <span>{{ tag.date }}</span>
              </div>
            </div>

            <!-- Message (if any) -->
            <p v-if="tag.message" class="text-xs text-zinc-300 font-sans leading-relaxed pl-1 break-words">
              {{ tag.message }}
            </p>

            <!-- Actions Row: Push, Delete -->
            <div class="flex items-center justify-end gap-2 pt-1 border-t border-zinc-900">
              <button
                type="button"
                :disabled="pushingTag === tag.name"
                class="flex items-center gap-1.5 px-2.5 py-1 rounded-xl text-xs font-semibold bg-sky-500/10 hover:bg-sky-500/20 text-sky-400 border border-sky-500/25 transition-all active:scale-95 disabled:opacity-50"
                title="Push tag to remote origin"
                @click="handlePushTag(tag)"
              >
                <Loader2 v-if="pushingTag === tag.name" class="w-3 h-3 animate-spin" />
                <UploadCloud v-else class="w-3 h-3" />
                <span>{{ pushingTag === tag.name ? 'Pushing...' : 'Push to Origin' }}</span>
              </button>

              <button
                type="button"
                class="p-1.5 rounded-xl text-zinc-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                title="Delete tag"
                @click="promptDeleteTag(tag)"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Custom Delete Tag Confirmation Dialog -->
    <div
      v-if="tagToDelete"
      class="viewport-fixed modal-overlay z-[60] bg-black/80 backdrop-blur-sm animate-fade-in"
      @click.self="tagToDelete = null"
    >
      <div class="modal-panel max-w-sm bg-zinc-900 border border-zinc-800 rounded-2xl shadow-2xl p-5 space-y-4 overflow-y-auto">
        <div class="flex items-start gap-3">
          <div class="p-2.5 rounded-xl bg-red-500/10 text-red-400 border border-red-500/20 shrink-0">
            <Trash2 class="w-5 h-5" />
          </div>
          <div class="space-y-1">
            <h3 class="text-base font-semibold text-zinc-100">Delete Tag</h3>
            <p class="text-sm text-zinc-400">
              Are you sure you want to delete tag <code class="text-emerald-400 font-mono font-bold">{{ tagToDelete.name }}</code>?
            </p>
          </div>
        </div>

        <!-- Optional Remote Delete Checkbox -->
        <label class="flex items-center gap-2 p-3 rounded-xl bg-zinc-950/60 border border-zinc-800 text-xs text-zinc-300 cursor-pointer select-none">
          <input
            v-model="deleteRemoteAlso"
            type="checkbox"
            class="w-4 h-4 rounded border-zinc-700 bg-zinc-900 text-red-500 focus:ring-0 focus:ring-offset-0"
          />
          <span>Also delete from remote (<code class="text-zinc-400 font-mono">origin</code>)</span>
        </label>

        <div class="flex items-center justify-end gap-2.5 pt-1">
          <button
            type="button"
            class="px-4 py-2 text-sm font-medium text-zinc-300 bg-zinc-800 hover:bg-zinc-700 rounded-xl transition-colors"
            @click="tagToDelete = null"
          >
            Cancel
          </button>
          <button
            type="button"
            :disabled="isDeleting"
            class="flex items-center gap-1.5 px-4 py-2 text-sm font-semibold rounded-xl bg-red-600 hover:bg-red-500 text-white shadow-sm transition-all active:scale-95 disabled:opacity-50"
            @click="confirmDeleteTag"
          >
            <Loader2 v-if="isDeleting" class="w-4 h-4 animate-spin" />
            <span>{{ isDeleting ? 'Deleting...' : 'Delete Tag' }}</span>
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
