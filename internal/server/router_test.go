package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"branch-office/internal/config"
	"branch-office/internal/git"
)

func setupTestRepo(t *testing.T) (string, *config.Store, *config.Repo) {
	tempDir, err := os.MkdirTemp("", "broffice-srv-test-*")
	if err != nil {
		t.Fatal(err)
	}

	gitDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "init", "-b", "main", gitDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git author for test repo so commit doesn't complain
	exec.Command("git", "-C", gitDir, "config", "user.name", "Test User").Run()
	exec.Command("git", "-C", gitDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", gitDir, "config", "commit.gpgSign", "false").Run()

	cfgPath := filepath.Join(tempDir, "config.json")
	store, err := config.NewStore(cfgPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	repo, err := store.AddRepo(gitDir)
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	return tempDir, store, repo
}

func TestRouterEndpoints(t *testing.T) {
	tempDir, store, repo := setupTestRepo(t)
	defer os.RemoveAll(tempDir)

	mockFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><body>Branch Office</body></html>")},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log('app')")},
	}

	srv := NewServer(store, mockFS)
	handler := srv.Routes()

	// 1. Test GET /api/repos
	t.Run("GET /api/repos", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/repos", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var repos []RepoWithMeta
		if err := json.NewDecoder(rec.Body).Decode(&repos); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(repos) != 1 || repos[0].ID != repo.ID {
			t.Fatalf("unexpected repos list: %+v", repos)
		}
	})

	// 2. Test GET /api/repos/{id}/status initially
	t.Run("GET /api/repos/{id}/status initial", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/status", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var status git.RepoStatus
		if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
			t.Fatalf("failed to decode status: %v", err)
		}
		if status.Branch != "main" {
			t.Errorf("expected branch main, got %s", status.Branch)
		}
		if len(status.Files) != 0 {
			t.Errorf("expected 0 files initially, got %d", len(status.Files))
		}
	})

	// 3. Create an untracked file and verify status & stage
	sampleFile := filepath.Join(repo.Path, "hello.txt")
	if err := os.WriteFile(sampleFile, []byte("line 1\nline 2\nline 3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("Status with untracked file", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/status", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var status git.RepoStatus
		json.NewDecoder(rec.Body).Decode(&status)
		if status.UntrackedCount != 1 {
			t.Fatalf("expected 1 untracked file, got %d", status.UntrackedCount)
		}
	})

	// 4. Test stage file
	t.Run("POST /api/repos/{id}/stage", func(t *testing.T) {
		body := `{"files": ["hello.txt"]}`
		req := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/stage", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		// Verify staged status
		req2 := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/status", nil)
		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, req2)
		var status git.RepoStatus
		json.NewDecoder(rec2.Body).Decode(&status)
		if status.StagedCount != 1 {
			t.Fatalf("expected 1 staged file, got %d", status.StagedCount)
		}
	})

	// 5. Test initial commit
	t.Run("POST /api/repos/{id}/commit", func(t *testing.T) {
		body := `{"message": "Initial test commit"}`
		req := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/commit", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	// 6. Test modifying file and diff hunks
	if err := os.WriteFile(sampleFile, []byte("line 1 modified\nline 2\nline 3\nline 4 added\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var hunkPatch string
	t.Run("GET /api/repos/{id}/diff", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/diff?file=hello.txt", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var diffs []git.FileDiff
		if err := json.NewDecoder(rec.Body).Decode(&diffs); err != nil {
			t.Fatalf("failed to decode diff: %v", err)
		}
		if len(diffs) != 1 || len(diffs[0].Hunks) == 0 {
			t.Fatalf("expected 1 diff with hunks, got %+v", diffs)
		}
		hunkPatch = diffs[0].Hunks[0].Patch
	})

	// 7. Test hunk staging & unstaging
	t.Run("Stage and unstage hunk", func(t *testing.T) {
		if hunkPatch == "" {
			t.Fatal("no hunk patch extracted")
		}

		// Stage hunk
		body := map[string]string{"patch": hunkPatch}
		jsonBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/stage-hunk", bytes.NewReader(jsonBytes))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 staging hunk, got %d: %s", rec.Code, rec.Body.String())
		}

		// Check staged
		reqStatus := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/status", nil)
		recStatus := httptest.NewRecorder()
		handler.ServeHTTP(recStatus, reqStatus)
		var status git.RepoStatus
		json.NewDecoder(recStatus.Body).Decode(&status)
		if status.StagedCount != 1 {
			t.Fatalf("expected 1 staged file after hunk stage, got %d", status.StagedCount)
		}

		// Unstage hunk
		reqUnstage := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/unstage-hunk", bytes.NewReader(jsonBytes))
		recUnstage := httptest.NewRecorder()
		handler.ServeHTTP(recUnstage, reqUnstage)
		if recUnstage.Code != http.StatusOK {
			t.Fatalf("expected 200 unstaging hunk, got %d: %s", recUnstage.Code, recUnstage.Body.String())
		}
	})

	// 8. Test file discard
	t.Run("POST /api/repos/{id}/discard", func(t *testing.T) {
		body := `{"files": ["hello.txt"]}`
		req := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/discard", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 discarding file, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify file restored to clean state
		content, err := os.ReadFile(sampleFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "line 1\nline 2\nline 3\n" {
			t.Fatalf("file content not restored: %s", string(content))
		}
	})

	// 9. Test SPA static files and fallback
	t.Run("SPA static routing", func(t *testing.T) {
		// Existing asset
		reqAsset := httptest.NewRequest("GET", "/assets/app.js", nil)
		recAsset := httptest.NewRecorder()
		handler.ServeHTTP(recAsset, reqAsset)
		if recAsset.Code != http.StatusOK || !strings.Contains(recAsset.Body.String(), "console.log") {
			t.Fatalf("expected app.js, got %d: %s", recAsset.Code, recAsset.Body.String())
		}

		// Non-existent route falls back to index.html
		reqFallback := httptest.NewRequest("GET", "/repo/some-id/settings", nil)
		recFallback := httptest.NewRecorder()
		handler.ServeHTTP(recFallback, reqFallback)
		if recFallback.Code != http.StatusOK || !strings.Contains(recFallback.Body.String(), "Branch Office") {
			t.Fatalf("expected SPA fallback index.html, got %d: %s", recFallback.Code, recFallback.Body.String())
		}
	})

	// 10. Test SSE live events stream
	t.Run("GET /api/repos/{id}/events live stream", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		req := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/events", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		done := make(chan struct{})
		go func() {
			handler.ServeHTTP(rec, req)
			close(done)
		}()

		// Allow watcher to initialize and emit connected
		time.Sleep(100 * time.Millisecond)

		// Modify file to trigger watcher
		if err := os.WriteFile(sampleFile, []byte("change for sse test\n"), 0644); err != nil {
			t.Fatal(err)
		}

		// Wait for debounce and broadcast (200ms debounce + buffer)
		time.Sleep(350 * time.Millisecond)

		cancel()
		<-done

		body := rec.Body.String()
		if !strings.Contains(body, "event: connected") {
			t.Fatalf("expected event: connected, got: %s", body)
		}
		if !strings.Contains(body, "event: change") {
			t.Fatalf("expected event: change, got: %s", body)
		}
	})

	// 11. Test Worktree endpoints
	t.Run("Worktree endpoints", func(t *testing.T) {
		// GET worktrees initial
		reqGet := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/worktrees", nil)
		recGet := httptest.NewRecorder()
		handler.ServeHTTP(recGet, reqGet)
		if recGet.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", recGet.Code)
		}
		var wts []git.Worktree
		json.NewDecoder(recGet.Body).Decode(&wts)
		if len(wts) != 1 || !wts[0].IsMain {
			t.Fatalf("expected 1 main worktree, got %+v", wts)
		}

		// POST worktrees
		wtPath := filepath.Join(tempDir, "wt-branch")
		postBody := fmt.Sprintf(`{"path": "%s", "branch": "feature-wt", "createBranch": true}`, wtPath)
		reqPost := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/worktrees", strings.NewReader(postBody))
		recPost := httptest.NewRecorder()
		handler.ServeHTTP(recPost, reqPost)
		if recPost.Code != http.StatusCreated {
			t.Fatalf("expected 201 creating worktree, got %d: %s", recPost.Code, recPost.Body.String())
		}

		// Verify worktree was auto-added to repos list
		reposAfter := store.ListRepos()
		if len(reposAfter) != 2 {
			t.Fatalf("expected 2 repos in store, got %d", len(reposAfter))
		}

		// DELETE worktree
		reqDel := httptest.NewRequest("DELETE", "/api/repos/"+repo.ID+"/worktrees?path="+wtPath, nil)
		recDel := httptest.NewRecorder()
		handler.ServeHTTP(recDel, reqDel)
		if recDel.Code != http.StatusOK {
			t.Fatalf("expected 200 deleting worktree, got %d: %s", recDel.Code, recDel.Body.String())
		}

		// Verify worktree list back to 1
		reqGet2 := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/worktrees", nil)
		recGet2 := httptest.NewRecorder()
		handler.ServeHTTP(recGet2, reqGet2)
		var wts2 []git.Worktree
		json.NewDecoder(recGet2.Body).Decode(&wts2)
		if len(wts2) != 1 {
			t.Fatalf("expected 1 worktree after deletion, got %d", len(wts2))
		}
	})

	t.Run("POST /api/repos/{id}/generate-commit-message_no_staged", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/generate-commit-message", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for no staged changes, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Git Branch Endpoints", func(t *testing.T) {
		// First commit something so HEAD is valid
		testFile := filepath.Join(repo.Path, "branch_test.txt")
		os.WriteFile(testFile, []byte("test"), 0644)
		exec.Command("git", "-C", repo.Path, "add", "branch_test.txt").Run()
		exec.Command("git", "-C", repo.Path, "commit", "-m", "commit for branches").Run()

		// 1. GET /api/repos/{id}/branches
		reqGet := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/branches", nil)
		recGet := httptest.NewRecorder()
		handler.ServeHTTP(recGet, reqGet)
		if recGet.Code != http.StatusOK {
			t.Fatalf("expected 200 listing branches, got %d: %s", recGet.Code, recGet.Body.String())
		}
		var branches []git.Branch
		if err := json.NewDecoder(recGet.Body).Decode(&branches); err != nil {
			t.Fatalf("failed to decode branches: %v", err)
		}
		if len(branches) < 1 {
			t.Fatalf("expected at least 1 branch, got %d", len(branches))
		}

		// 2. POST /api/repos/{id}/branches/create
		createPayload := `{"name": "feature-xyz"}`
		reqCreate := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/branches/create", strings.NewReader(createPayload))
		recCreate := httptest.NewRecorder()
		handler.ServeHTTP(recCreate, reqCreate)
		if recCreate.Code != http.StatusCreated {
			t.Fatalf("expected 201 creating branch, got %d: %s", recCreate.Code, recCreate.Body.String())
		}

		// 3. POST /api/repos/{id}/branches/checkout back to main
		checkoutPayload := `{"branch": "main"}`
		reqCheckout := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/branches/checkout", strings.NewReader(checkoutPayload))
		recCheckout := httptest.NewRecorder()
		handler.ServeHTTP(recCheckout, reqCheckout)
		if recCheckout.Code != http.StatusOK {
			t.Fatalf("expected 200 checking out branch, got %d: %s", recCheckout.Code, recCheckout.Body.String())
		}

		// 4. GET /api/repos/{id}/branches again - verify feature-xyz exists and main is current
		reqGet2 := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/branches", nil)
		recGet2 := httptest.NewRecorder()
		handler.ServeHTTP(recGet2, reqGet2)
		var branches2 []git.Branch
		json.NewDecoder(recGet2.Body).Decode(&branches2)
		var foundMain, foundNew bool
		for _, b := range branches2 {
			if b.Name == "main" {
				foundMain = true
				if !b.IsCurrent {
					t.Fatalf("expected main to be current")
				}
			}
			if b.Name == "feature-xyz" {
				foundNew = true
				if b.IsCurrent {
					t.Fatalf("expected feature-xyz not to be current")
				}
			}
		}
		if !foundMain || !foundNew {
			t.Fatalf("expected both main and feature-xyz in branches, got %+v", branches2)
		}
	})

	t.Run("GET /api/repos/{id}/last-commit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/last-commit", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 from last-commit endpoint, got %d: %s", rec.Code, rec.Body.String())
		}
		var res map[string]string
		json.NewDecoder(rec.Body).Decode(&res)
		if res["message"] != "commit for branches" {
			t.Fatalf("expected 'commit for branches', got '%s'", res["message"])
		}

		// Also verify status includes lastCommitMessage
		reqStatus := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/status", nil)
		recStatus := httptest.NewRecorder()
		handler.ServeHTTP(recStatus, reqStatus)
		if recStatus.Code != http.StatusOK {
			t.Fatalf("expected 200 from status, got %d", recStatus.Code)
		}
		var status git.RepoStatus
		json.NewDecoder(recStatus.Body).Decode(&status)
		if status.LastCommitMessage != "commit for branches" {
			t.Fatalf("expected status.LastCommitMessage 'commit for branches', got '%s'", status.LastCommitMessage)
		}
	})

	t.Run("Git Stash Endpoints", func(t *testing.T) {
		// List initially empty
		req := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/stashes", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 from stashes, got %d", rec.Code)
		}
		var stashes []git.StashItem
		json.NewDecoder(rec.Body).Decode(&stashes)
		if len(stashes) != 0 {
			t.Fatalf("expected 0 stashes, got %d", len(stashes))
		}

		// Create a file to stash
		stashFile := filepath.Join(repo.Path, "stash-test.txt")
		os.WriteFile(stashFile, []byte("stash content\n"), 0644)

		// POST /api/repos/{id}/stashes
		savePayload := `{"message":"test stash message","includeUntracked":true}`
		reqSave := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/stashes", strings.NewReader(savePayload))
		reqSave.Header.Set("Content-Type", "application/json")
		recSave := httptest.NewRecorder()
		handler.ServeHTTP(recSave, reqSave)
		if recSave.Code != http.StatusCreated {
			t.Fatalf("expected 201 from save stash, got %d: %s", recSave.Code, recSave.Body.String())
		}

		// GET /api/repos/{id}/stashes
		reqList := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/stashes", nil)
		recList := httptest.NewRecorder()
		handler.ServeHTTP(recList, reqList)
		if recList.Code != http.StatusOK {
			t.Fatalf("expected 200 from stashes list, got %d", recList.Code)
		}
		json.NewDecoder(recList.Body).Decode(&stashes)
		if len(stashes) != 1 {
			t.Fatalf("expected 1 stash, got %d", len(stashes))
		}
		if stashes[0].Message != "test stash message" {
			t.Fatalf("expected 'test stash message', got '%s'", stashes[0].Message)
		}

		// GET /api/repos/{id}/stashes/0/diff
		reqDiff := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/stashes/0/diff", nil)
		recDiff := httptest.NewRecorder()
		handler.ServeHTTP(recDiff, reqDiff)
		if recDiff.Code != http.StatusOK {
			t.Fatalf("expected 200 from stash diff, got %d: %s", recDiff.Code, recDiff.Body.String())
		}
		var diffRes map[string]string
		json.NewDecoder(recDiff.Body).Decode(&diffRes)
		if !strings.Contains(diffRes["diff"], "stash content") {
			t.Fatalf("expected diff to contain 'stash content', got '%s'", diffRes["diff"])
		}

		// POST /api/repos/{id}/stashes/0/apply
		reqApply := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/stashes/0/apply", nil)
		recApply := httptest.NewRecorder()
		handler.ServeHTTP(recApply, reqApply)
		if recApply.Code != http.StatusOK {
			t.Fatalf("expected 200 from stash apply, got %d: %s", recApply.Code, recApply.Body.String())
		}

		// Reset working tree
		os.Remove(stashFile)
		client := git.NewClient(repo.Path)
		_, _ = client.Run("checkout", ".")

		// POST /api/repos/{id}/stashes/0/pop
		reqPop := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/stashes/0/pop", nil)
		recPop := httptest.NewRecorder()
		handler.ServeHTTP(recPop, reqPop)
		if recPop.Code != http.StatusOK {
			t.Fatalf("expected 200 from stash pop, got %d: %s", recPop.Code, recPop.Body.String())
		}

		// Clean up restored file
		os.Remove(stashFile)
	})

	t.Run("Git Tag Endpoints", func(t *testing.T) {
		// List initially empty
		req := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/tags", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 from tags list, got %d", rec.Code)
		}
		var tags []git.TagItem
		json.NewDecoder(rec.Body).Decode(&tags)
		if len(tags) != 0 {
			t.Fatalf("expected 0 tags, got %d", len(tags))
		}

		// POST /api/repos/{id}/tags (create annotated tag)
		createPayload := `{"name":"v1.0.0","message":"release v1.0.0","push":false}`
		reqCreate := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/tags", strings.NewReader(createPayload))
		reqCreate.Header.Set("Content-Type", "application/json")
		recCreate := httptest.NewRecorder()
		handler.ServeHTTP(recCreate, reqCreate)
		if recCreate.Code != http.StatusOK {
			t.Fatalf("expected 200 from create tag, got %d: %s", recCreate.Code, recCreate.Body.String())
		}

		// POST /api/repos/{id}/tags (create lightweight tag)
		createLwPayload := `{"name":"v1.0.1-lw","push":false}`
		reqCreateLw := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/tags", strings.NewReader(createLwPayload))
		reqCreateLw.Header.Set("Content-Type", "application/json")
		recCreateLw := httptest.NewRecorder()
		handler.ServeHTTP(recCreateLw, reqCreateLw)
		if recCreateLw.Code != http.StatusOK {
			t.Fatalf("expected 200 from create lightweight tag, got %d: %s", recCreateLw.Code, recCreateLw.Body.String())
		}

		// GET /api/repos/{id}/tags
		reqList := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/tags", nil)
		recList := httptest.NewRecorder()
		handler.ServeHTTP(recList, reqList)
		if recList.Code != http.StatusOK {
			t.Fatalf("expected 200 from tags list, got %d", recList.Code)
		}
		json.NewDecoder(recList.Body).Decode(&tags)
		if len(tags) != 2 {
			t.Fatalf("expected 2 tags, got %d", len(tags))
		}

		// Check status has TagCount == 2
		reqStatus := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/status", nil)
		recStatus := httptest.NewRecorder()
		handler.ServeHTTP(recStatus, reqStatus)
		var repoStatus git.RepoStatus
		json.NewDecoder(recStatus.Body).Decode(&repoStatus)
		if repoStatus.TagCount != 2 {
			t.Fatalf("expected repoStatus.TagCount == 2, got %d", repoStatus.TagCount)
		}

		// DELETE /api/repos/{id}/tags/v1.0.1-lw
		reqDel := httptest.NewRequest("DELETE", "/api/repos/"+repo.ID+"/tags/v1.0.1-lw", nil)
		recDel := httptest.NewRecorder()
		handler.ServeHTTP(recDel, reqDel)
		if recDel.Code != http.StatusOK {
			t.Fatalf("expected 200 from delete tag, got %d: %s", recDel.Code, recDel.Body.String())
		}

		// Verify 1 tag remains
		reqListAfter := httptest.NewRequest("GET", "/api/repos/"+repo.ID+"/tags", nil)
		recListAfter := httptest.NewRecorder()
		handler.ServeHTTP(recListAfter, reqListAfter)
		var tagsAfter []git.TagItem
		json.NewDecoder(recListAfter.Body).Decode(&tagsAfter)
		if len(tagsAfter) != 1 {
			t.Fatalf("expected 1 tag after delete, got %d", len(tagsAfter))
		}
		if tagsAfter[0].Name != "v1.0.0" {
			t.Fatalf("expected remaining tag 'v1.0.0', got '%s'", tagsAfter[0].Name)
		}
	})
}

