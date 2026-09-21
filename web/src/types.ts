export interface Repo {
  id: string
  name: string
  path: string
  branch?: string
  clean?: boolean
  dirtyCount?: number
  error?: string
}

export interface Worktree {
  path: string
  branch: string
  head: string
  isMain: boolean
  isLocked: boolean
  lockReason?: string
  prunable?: boolean
}

export interface FileStatus {
  path: string
  origPath?: string
  stagedStatus: 'none' | 'modified' | 'added' | 'deleted' | 'renamed' | 'typechange' | 'conflicted' | 'unknown'
  unstagedStatus: 'none' | 'modified' | 'deleted' | 'untracked' | 'typechange' | 'conflicted' | 'unknown'
  isStaged: boolean
  isUnstaged: boolean
  isUntracked: boolean
  isConflicted: boolean
}

export interface RepoStatus {
  branch: string
  upstream: string
  ahead: number
  behind: number
  hasHead: boolean
  lastCommitMessage?: string
  files: FileStatus[]
  stagedCount: number
  unstagedCount: number
  untrackedCount: number
  conflictedCount: number
  stashCount?: number
  tagCount?: number
}

export interface DiffLine {
  type: 'context' | 'addition' | 'deletion' | 'meta'
  oldLine?: number
  newLine?: number
  content: string
}

export interface Hunk {
  index: number
  header: string
  oldStart: number
  oldLength: number
  newStart: number
  newLength: number
  section?: string
  lines: DiffLine[]
  patch: string
}

export interface FileDiff {
  oldPath: string
  newPath: string
  isNew: boolean
  isDeleted: boolean
  isRenamed: boolean
  isBinary: boolean
  additions: number
  deletions: number
  hunks: Hunk[]
  fileHeader: string
}

export interface PRDetails {
  number?: number
  title: string
  state?: string
  url: string
  body?: string
  author?: {
    login: string
  }
  headRefName?: string
  baseRefName?: string
}

export interface PRStatus {
  installed: boolean
  exists: boolean
  pr?: PRDetails
  message?: string
}

export interface Branch {
  name: string
  isCurrent: boolean
  isRemote: boolean
  upstream?: string
  commitHash?: string
  commitMsg?: string
}

export interface StashItem {
  index: number
  ref: string
  hash: string
  message: string
  branch: string
  date: string
}

export interface TagItem {
  name: string
  commitHash: string
  date: string
  isAnnotated: boolean
  message: string
}

export interface CommitItem {
  hash: string
  shortHash: string
  author: string
  email: string
  timestamp: number
  date: string
  relativeDate: string
  subject: string
  body: string
  refs: string[]
}

export interface CommitDetails {
  commit: CommitItem
  diffs: FileDiff[]
}
