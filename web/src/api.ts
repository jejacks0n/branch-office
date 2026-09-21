import type { Repo, RepoStatus, FileDiff, PRStatus, PRDetails, Worktree, Branch, StashItem, TagItem } from './types'

const BASE_URL = '/api'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })

  if (!res.ok) {
    let errMessage = `HTTP ${res.status}`
    try {
      const data = await res.json()
      if (data.error) errMessage = data.error
    } catch {
      // ignore
    }
    throw new Error(errMessage)
  }

  return res.json()
}

export const api = {
  // Repositories
  async getRepos(): Promise<Repo[]> {
    return request<Repo[]>('/repos')
  },

  async addRepo(path: string): Promise<Repo> {
    return request<Repo>('/repos', {
      method: 'POST',
      body: JSON.stringify({ path }),
    })
  },

  async removeRepo(id: string): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${id}`, {
      method: 'DELETE',
    })
  },

  // Repo Git status
  async getStatus(repoId: string): Promise<RepoStatus> {
    return request<RepoStatus>(`/repos/${repoId}/status`)
  },

  // Diff
  async getDiff(repoId: string, file?: string, staged = false, untracked = false): Promise<FileDiff[]> {
    const params = new URLSearchParams()
    if (file) params.set('file', file)
    if (staged) params.set('staged', 'true')
    if (untracked) params.set('untracked', 'true')
    const qs = params.toString() ? `?${params.toString()}` : ''
    return request<FileDiff[]>(`/repos/${repoId}/diff${qs}`)
  },

  // Staging
  async stageFiles(repoId: string, files: string[] = [], all = false): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/stage`, {
      method: 'POST',
      body: JSON.stringify({ files, all }),
    })
  },

  async unstageFiles(repoId: string, files: string[] = [], all = false): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/unstage`, {
      method: 'POST',
      body: JSON.stringify({ files, all }),
    })
  },

  // Hunks
  async stageHunk(repoId: string, patch: string): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/stage-hunk`, {
      method: 'POST',
      body: JSON.stringify({ patch }),
    })
  },

  async unstageHunk(repoId: string, patch: string): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/unstage-hunk`, {
      method: 'POST',
      body: JSON.stringify({ patch }),
    })
  },

  // Discards
  async discardFiles(repoId: string, files: string[] = [], all = false): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/discard`, {
      method: 'POST',
      body: JSON.stringify({ files, all }),
    })
  },

  async discardHunk(repoId: string, patch: string): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/discard-hunk`, {
      method: 'POST',
      body: JSON.stringify({ patch }),
    })
  },

  // Commit
  async commit(repoId: string, message: string, amend = false): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/commit`, {
      method: 'POST',
      body: JSON.stringify({ message, amend }),
    })
  },

  async generateCommitMessage(repoId: string, hint?: string): Promise<{ message: string }> {
    return request<{ message: string }>(`/repos/${repoId}/generate-commit-message`, {
      method: 'POST',
      body: JSON.stringify({ hint: hint || '' }),
    })
  },

  async getLastCommitMessage(repoId: string): Promise<{ message: string }> {
    return request<{ message: string }>(`/repos/${repoId}/last-commit`)
  },

  // Push
  async push(repoId: string, forceWithLease = false, setUpstream = false): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/push`, {
      method: 'POST',
      body: JSON.stringify({ forceWithLease, setUpstream }),
    })
  },

  // GitHub PR
  async getPRStatus(repoId: string): Promise<PRStatus> {
    return request<PRStatus>(`/repos/${repoId}/pr`)
  },

  async createPR(repoId: string, title: string, body: string, draft = false): Promise<PRDetails> {
    return request<PRDetails>(`/repos/${repoId}/pr`, {
      method: 'POST',
      body: JSON.stringify({ title, body, draft }),
    })
  },

  // Worktrees
  async getWorktrees(repoId: string): Promise<Worktree[]> {
    const res = await request<Worktree[]>(`/repos/${repoId}/worktrees`)
    return Array.isArray(res) ? res : []
  },

  async addWorktree(repoId: string, path: string, branch: string, createBranch = false): Promise<Worktree> {
    return request<Worktree>(`/repos/${repoId}/worktrees`, {
      method: 'POST',
      body: JSON.stringify({ path, branch, createBranch }),
    })
  },

  async removeWorktree(repoId: string, path: string, force = false): Promise<{ success: boolean }> {
    const params = new URLSearchParams({ path })
    if (force) params.set('force', 'true')
    return request<{ success: boolean }>(`/repos/${repoId}/worktrees?${params.toString()}`, {
      method: 'DELETE',
    })
  },

  // Branches
  async getBranches(repoId: string): Promise<Branch[]> {
    const res = await request<Branch[]>(`/repos/${repoId}/branches`)
    return Array.isArray(res) ? res : []
  },

  async checkoutBranch(repoId: string, branch: string): Promise<{ success: boolean; branch: string }> {
    return request<{ success: boolean; branch: string }>(`/repos/${repoId}/branches/checkout`, {
      method: 'POST',
      body: JSON.stringify({ branch }),
    })
  },

  async createBranch(repoId: string, name: string, startPoint?: string): Promise<{ success: boolean; branch: string }> {
    return request<{ success: boolean; branch: string }>(`/repos/${repoId}/branches/create`, {
      method: 'POST',
      body: JSON.stringify({ name, startPoint: startPoint || '' }),
    })
  },

  // Stash
  async getStashes(repoId: string): Promise<StashItem[]> {
    const res = await request<StashItem[]>(`/repos/${repoId}/stashes`)
    return Array.isArray(res) ? res : []
  },

  async createStash(repoId: string, message = '', includeUntracked = true): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/stashes`, {
      method: 'POST',
      body: JSON.stringify({ message, includeUntracked }),
    })
  },

  async popStash(repoId: string, index = 0): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/stashes/${index}/pop`, {
      method: 'POST',
    })
  },

  async applyStash(repoId: string, index = 0): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/stashes/${index}/apply`, {
      method: 'POST',
    })
  },

  async dropStash(repoId: string, index: number): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/stashes/${index}`, {
      method: 'DELETE',
    })
  },

  async clearStashes(repoId: string): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/stashes`, {
      method: 'DELETE',
    })
  },

  async getStashDiff(repoId: string, index: number): Promise<{ diff: string }> {
    return request<{ diff: string }>(`/repos/${repoId}/stashes/${index}/diff`)
  },

  // Tags
  async getTags(repoId: string): Promise<TagItem[]> {
    const res = await request<TagItem[]>(`/repos/${repoId}/tags`)
    return Array.isArray(res) ? res : []
  },

  async createTag(
    repoId: string,
    options: { name: string; message?: string; push?: boolean }
  ): Promise<{ success: boolean; name: string }> {
    return request<{ success: boolean; name: string }>(`/repos/${repoId}/tags`, {
      method: 'POST',
      body: JSON.stringify(options),
    })
  },

  async pushTag(repoId: string, tagName: string): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(`/repos/${repoId}/tags/${encodeURIComponent(tagName)}/push`, {
      method: 'POST',
    })
  },

  async deleteTag(repoId: string, tagName: string, remote = false): Promise<{ success: boolean }> {
    return request<{ success: boolean }>(
      `/repos/${repoId}/tags/${encodeURIComponent(tagName)}${remote ? '?remote=true' : ''}`,
      {
        method: 'DELETE',
      }
    )
  },
}
