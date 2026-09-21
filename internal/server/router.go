package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"branch-office/internal/config"
	"branch-office/internal/git"
	"branch-office/internal/github"
)

type Server struct {
	store    *config.Store
	staticFS fs.FS
	watchers *WatcherManager
}

func NewServer(store *config.Store, staticFS fs.FS) *Server {
	return &Server{
		store:    store,
		staticFS: staticFS,
		watchers: NewWatcherManager(),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Repository manager endpoints
	mux.HandleFunc("GET /api/repos", s.handleListRepos)
	mux.HandleFunc("POST /api/repos", s.handleAddRepo)
	mux.HandleFunc("DELETE /api/repos/{id}", s.handleRemoveRepo)

	// Git endpoints for specific repository
	mux.HandleFunc("GET /api/repos/{id}/status", s.withRepo(s.handleStatus))
	mux.HandleFunc("GET /api/repos/{id}/diff", s.withRepo(s.handleDiff))
	mux.HandleFunc("GET /api/repos/{id}/events", s.withRepo(s.handleRepoEvents))
	mux.HandleFunc("POST /api/repos/{id}/stage", s.withRepo(s.handleStage))
	mux.HandleFunc("POST /api/repos/{id}/unstage", s.withRepo(s.handleUnstage))
	mux.HandleFunc("POST /api/repos/{id}/stage-hunk", s.withRepo(s.handleStageHunk))
	mux.HandleFunc("POST /api/repos/{id}/unstage-hunk", s.withRepo(s.handleUnstageHunk))
	mux.HandleFunc("POST /api/repos/{id}/discard", s.withRepo(s.handleDiscard))
	mux.HandleFunc("POST /api/repos/{id}/discard-hunk", s.withRepo(s.handleDiscardHunk))
	mux.HandleFunc("POST /api/repos/{id}/commit", s.withRepo(s.handleCommit))
	mux.HandleFunc("GET /api/repos/{id}/last-commit", s.withRepo(s.handleGetLastCommitMessage))
	mux.HandleFunc("POST /api/repos/{id}/generate-commit-message", s.withRepo(s.handleGenerateCommitMessage))
	mux.HandleFunc("POST /api/repos/{id}/push", s.withRepo(s.handlePush))
	mux.HandleFunc("POST /api/repos/{id}/pull", s.withRepo(s.handlePull))
	mux.HandleFunc("POST /api/repos/{id}/fetch", s.withRepo(s.handleFetch))

	// Git Commit History endpoints
	mux.HandleFunc("GET /api/repos/{id}/commits", s.withRepo(s.handleListCommits))
	mux.HandleFunc("GET /api/repos/{id}/commits/{hash}", s.withRepo(s.handleGetCommit))
	mux.HandleFunc("GET /api/repos/{id}/commits/{hash}/diff", s.withRepo(s.handleGetCommitDiff))

	// GitHub PR endpoints
	mux.HandleFunc("GET /api/repos/{id}/pr", s.withRepo(s.handlePRStatus))
	mux.HandleFunc("POST /api/repos/{id}/pr", s.withRepo(s.handlePRCreate))

	// Git Worktree endpoints
	mux.HandleFunc("GET /api/repos/{id}/worktrees", s.withRepo(s.handleListWorktrees))
	mux.HandleFunc("POST /api/repos/{id}/worktrees", s.withRepo(s.handleAddWorktree))
	mux.HandleFunc("DELETE /api/repos/{id}/worktrees", s.withRepo(s.handleRemoveWorktree))

	// Git Branch endpoints
	mux.HandleFunc("GET /api/repos/{id}/branches", s.withRepo(s.handleListBranches))
	mux.HandleFunc("POST /api/repos/{id}/branches/checkout", s.withRepo(s.handleCheckoutBranch))
	mux.HandleFunc("POST /api/repos/{id}/branches/create", s.withRepo(s.handleCreateBranch))

	// Git Stash endpoints
	mux.HandleFunc("GET /api/repos/{id}/stashes", s.withRepo(s.handleListStashes))
	mux.HandleFunc("POST /api/repos/{id}/stashes", s.withRepo(s.handleSaveStash))
	mux.HandleFunc("POST /api/repos/{id}/stashes/{index}/pop", s.withRepo(s.handlePopStash))
	mux.HandleFunc("POST /api/repos/{id}/stashes/{index}/apply", s.withRepo(s.handleApplyStash))
	mux.HandleFunc("DELETE /api/repos/{id}/stashes/{index}", s.withRepo(s.handleDropStash))
	mux.HandleFunc("DELETE /api/repos/{id}/stashes", s.withRepo(s.handleClearStashes))
	mux.HandleFunc("GET /api/repos/{id}/stashes/{index}/diff", s.withRepo(s.handleGetStashDiff))

	// Git Tag endpoints
	mux.HandleFunc("GET /api/repos/{id}/tags", s.withRepo(s.handleListTags))
	mux.HandleFunc("POST /api/repos/{id}/tags", s.withRepo(s.handleCreateTag))
	mux.HandleFunc("POST /api/repos/{id}/tags/{name}/push", s.withRepo(s.handlePushTag))
	mux.HandleFunc("DELETE /api/repos/{id}/tags/{name}", s.withRepo(s.handleDeleteTag))

	// Static assets and SPA fallback
	if s.staticFS != nil {
		fileServer := http.FileServer(http.FS(s.staticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}

			// Check if file exists in staticFS
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}

			f, err := s.staticFS.Open(path)
			if err == nil {
				f.Close()
				if path == "index.html" {
					w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Set("Pragma", "no-cache")
					w.Header().Set("Expires", "0")
				} else if strings.HasPrefix(path, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			}

			// SPA fallback: serve index.html
			indexFile, err := s.staticFS.Open("index.html")
			if err == nil {
				defer indexFile.Close()
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
				io.Copy(w, indexFile)
				return
			}

			http.NotFound(w, r)
		})
	}

	return s.loggingMiddleware(s.corsMiddleware(mux))
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Suppress persistent SSE streaming connection from cluttering terminal logs
		isStreaming := strings.HasSuffix(r.URL.Path, "/events")

		if !isStreaming {
			log.Printf("[HTTP] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

type RepoWithMeta struct {
	config.Repo
	Branch     string `json:"branch,omitempty"`
	Clean      bool   `json:"clean"`
	DirtyCount int    `json:"dirtyCount"`
	Error      string `json:"error,omitempty"`
}

func (s *Server) handleListRepos(w http.ResponseWriter, r *http.Request) {
	repos := s.store.ListRepos()
	result := make([]RepoWithMeta, len(repos))

	for i, r := range repos {
		item := RepoWithMeta{Repo: r, Clean: true}
		client := git.NewClient(r.Path).
			WithSSHKey(s.store.GetSSHKey()).
			WithDisableSigning(s.store.GetDisableSigning())
		status, err := client.Status()
		if err != nil {
			item.Error = err.Error()
		} else {
			item.Branch = status.Branch
			totalDirty := status.StagedCount + status.UnstagedCount + status.UntrackedCount
			item.DirtyCount = totalDirty
			item.Clean = (totalDirty == 0)
		}
		result[i] = item
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAddRepo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	repo, err := s.store.AddRepo(body.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, repo)
}

func (s *Server) handleRemoveRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.RemoveRepo(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

type repoHandlerFunc func(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client)

func (s *Server) withRepo(handler repoHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		repo, err := s.store.GetRepo(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}
		client := git.NewClient(repo.Path).
			WithSSHKey(s.store.GetSSHKey()).
			WithDisableSigning(s.store.GetDisableSigning())
		handler(w, r, repo, client)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	status, err := client.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	filePath := r.URL.Query().Get("file")
	staged := r.URL.Query().Get("staged") == "true"
	untracked := r.URL.Query().Get("untracked") == "true"

	diffs, err := client.GetDiff(filePath, staged, untracked)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, diffs)
}

func (s *Server) handleRepoEvents(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusBadRequest, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, unsubscribe, err := s.watchers.Subscribe(repo.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer unsubscribe()

	// Initial connected event
	fmt.Fprintf(w, "event: connected\ndata: {\"repoId\":\"%s\"}\n\n", repo.ID)
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			// Heartbeat comment
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case <-ch:
			// Live file or git change detected
			log.Printf("[SSE] File/Git change broadcast for repo %s (%s)", repo.Name, repo.ID)
			fmt.Fprintf(w, "event: change\ndata: {\"repoId\":\"%s\"}\n\n", repo.ID)
			flusher.Flush()
		}
	}
}

func (s *Server) handleStage(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Files []string `json:"files"`
		All   bool     `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var err error
	if body.All {
		err = client.StageAll()
	} else {
		err = client.StageFiles(body.Files)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleUnstage(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Files []string `json:"files"`
		All   bool     `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var err error
	if body.All {
		err = client.UnstageAll()
	} else {
		err = client.UnstageFiles(body.Files)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleStageHunk(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Patch string `json:"patch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := client.StageHunk(body.Patch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleUnstageHunk(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Patch string `json:"patch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := client.UnstageHunk(body.Patch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleDiscard(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Files []string `json:"files"`
		All   bool     `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var err error
	if body.All {
		err = client.DiscardAll()
	} else {
		err = client.DiscardFiles(body.Files)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleDiscardHunk(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Patch string `json:"patch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := client.DiscardHunk(body.Patch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleCommit(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Message string `json:"message"`
		Amend   bool   `json:"amend"`
		NoSign  bool   `json:"noSign"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.NoSign {
		client.WithDisableSigning(true)
	}

	if err := client.Commit(body.Message, body.Amend); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleGetLastCommitMessage(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	msg := client.GetLastCommitMessage()
	writeJSON(w, http.StatusOK, map[string]string{"message": msg})
}

func (s *Server) handleGenerateCommitMessage(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	diff, err := client.Run("diff", "--cached")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get staged diff: "+err.Error())
		return
	}
	if strings.TrimSpace(diff) == "" {
		writeError(w, http.StatusBadRequest, "no staged changes to generate commit message from")
		return
	}

	var body struct {
		Hint string `json:"hint"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	ghClient := github.NewClient(repo.Path)
	msg, err := ghClient.GenerateCommitMessage(r.Context(), diff, body.Hint)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": msg,
	})
}

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		ForceWithLease bool `json:"forceWithLease"`
		SetUpstream    bool `json:"setUpstream"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := client.Push(body.ForceWithLease, body.SetUpstream); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Rebase      bool `json:"rebase"`
		SetUpstream bool `json:"setUpstream"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	if err := client.Pull(body.Rebase, body.SetUpstream); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleFetch(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	if err := client.Fetch(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handlePRStatus(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	ghClient := github.NewClient(repo.Path)
	status, err := ghClient.GetPRStatus()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handlePRCreate(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		Draft bool   `json:"draft"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ghClient := github.NewClient(repo.Path)
	pr, err := ghClient.CreatePR(body.Title, body.Body, body.Draft)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, pr)
}

func (s *Server) handleListWorktrees(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	worktrees, err := client.ListWorktrees()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, worktrees)
}

func (s *Server) handleAddWorktree(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Path         string `json:"path"`
		Branch       string `json:"branch"`
		CreateBranch bool   `json:"createBranch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	wt, err := client.AddWorktree(body.Path, body.Branch, body.CreateBranch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Auto-register the new worktree in the config store so it is immediately accessible
	_, _ = s.store.AddRepo(wt.Path)

	writeJSON(w, http.StatusCreated, wt)
}

func (s *Server) handleRemoveWorktree(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	targetPath := r.URL.Query().Get("path")
	if targetPath == "" {
		writeError(w, http.StatusBadRequest, "path parameter required")
		return
	}
	force := r.URL.Query().Get("force") == "true"

	if err := client.RemoveWorktree(targetPath, force); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Also untrack from store if present
	repoID := config.HashPath(targetPath)
	_ = s.store.RemoveRepo(repoID)

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleListBranches(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	branches, err := client.ListBranches()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, branches)
}

func (s *Server) handleCheckoutBranch(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Branch string `json:"branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(body.Branch) == "" {
		writeError(w, http.StatusBadRequest, "branch name is required")
		return
	}

	if err := client.CheckoutBranch(body.Branch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"branch":  client.GetCurrentBranch(),
	})
}

func (s *Server) handleCreateBranch(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Name       string `json:"name"`
		StartPoint string `json:"startPoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "branch name is required")
		return
	}

	if err := client.CreateBranch(body.Name, body.StartPoint); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"branch":  client.GetCurrentBranch(),
	})
}

func (s *Server) handleListStashes(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	stashes, err := client.ListStashes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stashes == nil {
		stashes = []git.StashItem{}
	}
	writeJSON(w, http.StatusOK, stashes)
}

func (s *Server) handleSaveStash(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var body struct {
		Message          string `json:"message"`
		IncludeUntracked bool   `json:"includeUntracked"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	if err := client.SaveStash(body.Message, body.IncludeUntracked); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"success": true})
}

func (s *Server) handlePopStash(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	idxStr := r.PathValue("index")
	index, err := strconv.Atoi(idxStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid stash index")
		return
	}

	if err := client.PopStash(index); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleApplyStash(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	idxStr := r.PathValue("index")
	index, err := strconv.Atoi(idxStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid stash index")
		return
	}

	if err := client.ApplyStash(index); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleDropStash(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	idxStr := r.PathValue("index")
	index, err := strconv.Atoi(idxStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid stash index")
		return
	}

	if err := client.DropStash(index); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleClearStashes(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	if err := client.ClearStashes(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleGetStashDiff(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	idxStr := r.PathValue("index")
	index, err := strconv.Atoi(idxStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid stash index")
		return
	}

	diff, err := client.GetStashDiff(index)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": diff})
}

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	tags, err := client.ListTags()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tags == nil {
		tags = []git.TagItem{}
	}
	writeJSON(w, http.StatusOK, tags)
}

type createTagRequest struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Push    bool   `json:"push"`
}

func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	var req createTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "tag name cannot be empty")
		return
	}

	if err := client.CreateTag(req.Name, req.Message, ""); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Push {
		if err := client.PushTag(req.Name); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("tag created locally, but failed to push to remote: %v", err))
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "name": req.Name})
}

func (s *Server) handlePushTag(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "tag name is required")
		return
	}

	if err := client.PushTag(name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "tag name is required")
		return
	}

	remote := r.URL.Query().Get("remote") == "true" || r.URL.Query().Get("remote") == "1"

	if err := client.DeleteTag(name, remote); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleListCommits(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	limit := 30
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	skip := 0
	if sk := r.URL.Query().Get("skip"); sk != "" {
		if parsed, err := strconv.Atoi(sk); err == nil && parsed >= 0 {
			skip = parsed
		}
	}

	opts := git.LogOptions{
		Limit:  limit,
		Skip:   skip,
		Ref:    r.URL.Query().Get("ref"),
		Path:   r.URL.Query().Get("path"),
		Search: r.URL.Query().Get("search"),
	}

	commits, err := client.GetCommits(opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, commits)
}

func (s *Server) handleGetCommit(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	hash := r.PathValue("hash")
	if hash == "" {
		writeError(w, http.StatusBadRequest, "commit hash required")
		return
	}

	commit, diffs, err := client.GetCommit(hash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"commit": commit,
		"diffs":  diffs,
	})
}

func (s *Server) handleGetCommitDiff(w http.ResponseWriter, r *http.Request, repo *config.Repo, client *git.Client) {
	hash := r.PathValue("hash")
	if hash == "" {
		writeError(w, http.StatusBadRequest, "commit hash required")
		return
	}

	filePath := r.URL.Query().Get("file")
	diffs, err := client.GetCommitDiff(hash, filePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, diffs)
}
