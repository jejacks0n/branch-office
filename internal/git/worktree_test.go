package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseWorktreeList(t *testing.T) {
	fixture := `worktree /path/to/main-repo
HEAD 77c6c04c5521bbaf1bf7eea7819a48dd115e3caa
branch refs/heads/main

worktree /path/to/wt-feature
HEAD 77c6c04c5521bbaf1bf7eea7819a48dd115e3caa
branch refs/heads/feature

worktree /path/to/wt-locked
HEAD 1234567890abcdef
branch refs/heads/locked-branch
locked work in progress
`

	worktrees := ParseWorktreeList(fixture)
	if len(worktrees) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(worktrees))
	}

	if !worktrees[0].IsMain || worktrees[0].Branch != "main" {
		t.Errorf("unexpected main worktree: %+v", worktrees[0])
	}

	if worktrees[1].IsMain || worktrees[1].Branch != "feature" || worktrees[1].Path != "/path/to/wt-feature" {
		t.Errorf("unexpected linked worktree: %+v", worktrees[1])
	}

	if !worktrees[2].IsLocked || worktrees[2].LockReason != "work in progress" {
		t.Errorf("unexpected locked worktree: %+v", worktrees[2])
	}
}

func TestWorktreeOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "broffice-wt-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Initialize git repo with commit
	exec.Command("git", "init", "-b", "main", repoDir).Run()
	exec.Command("git", "-C", repoDir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", repoDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", repoDir, "config", "commit.gpgSign", "false").Run()

	testFile := filepath.Join(repoDir, "init.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	exec.Command("git", "-C", repoDir, "add", ".").Run()
	exec.Command("git", "-C", repoDir, "commit", "-m", "init").Run()

	client := NewClient(repoDir)

	// Verify GitDir
	gitDir, err := client.GetGitDir()
	if err != nil || !stringsHasSuffix(gitDir, ".git") {
		t.Fatalf("unexpected git dir: %s (err: %v)", gitDir, err)
	}

	// List initially
	wts, err := client.ListWorktrees()
	if err != nil || len(wts) != 1 {
		t.Fatalf("expected 1 initial worktree, got %d (err: %v)", len(wts), err)
	}

	// Add a new worktree
	wtPath := filepath.Join(tempDir, "wt-feature")
	newWt, err := client.AddWorktree(wtPath, "feature-1", true)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}
	if newWt.Branch != "feature-1" {
		t.Errorf("expected branch feature-1, got %s", newWt.Branch)
	}

	// Verify list now has 2 worktrees
	wts, err = client.ListWorktrees()
	if err != nil || len(wts) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(wts))
	}

	// Remove worktree
	if err := client.RemoveWorktree(wtPath, false); err != nil {
		t.Fatalf("failed to remove worktree: %v", err)
	}

	wts, err = client.ListWorktrees()
	if err != nil || len(wts) != 1 {
		t.Fatalf("expected 1 worktree after removal, got %d", len(wts))
	}
}

func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
