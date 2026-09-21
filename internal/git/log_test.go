package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseCommitItems(t *testing.T) {
	raw := "1a4007dd24813bc5a126feb3963a2fd3e6b67d87\x1f1a4007d\x1fJohn Doe\x1fjohn@example.com\x1f1789995122\x1f2026-09-21 06:52:02 -0600\x1f10 minutes ago\x1ffeat(git): add pull support\x1fThis is the commit body\nwith multiple lines\x1fHEAD -> main, origin/main\x1e"

	items := parseCommitItems(raw)
	if len(items) != 1 {
		t.Fatalf("expected 1 commit item, got %d", len(items))
	}

	item := items[0]
	if item.Hash != "1a4007dd24813bc5a126feb3963a2fd3e6b67d87" {
		t.Errorf("unexpected hash: %s", item.Hash)
	}
	if item.ShortHash != "1a4007d" {
		t.Errorf("unexpected shortHash: %s", item.ShortHash)
	}
	if item.Author != "John Doe" {
		t.Errorf("unexpected author: %s", item.Author)
	}
	if item.Email != "john@example.com" {
		t.Errorf("unexpected email: %s", item.Email)
	}
	if item.Timestamp != 1789995122 {
		t.Errorf("unexpected timestamp: %d", item.Timestamp)
	}
	if item.Subject != "feat(git): add pull support" {
		t.Errorf("unexpected subject: %s", item.Subject)
	}
	if item.Body != "This is the commit body\nwith multiple lines" {
		t.Errorf("unexpected body: %s", item.Body)
	}
	if len(item.Refs) != 2 || item.Refs[0] != "HEAD -> main" || item.Refs[1] != "origin/main" {
		t.Errorf("unexpected refs: %+v", item.Refs)
	}
}

func TestCommitOperations(t *testing.T) {
	tmpDir := t.TempDir()
	initCmd := exec.Command("git", "init", tmpDir)
	if err := initCmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	client := NewClient(tmpDir)

	// Configure local user to avoid signing issues during test
	_, _ = client.Run("config", "user.name", "Test User")
	_, _ = client.Run("config", "user.email", "test@example.com")
	_, _ = client.Run("config", "commit.gpgsign", "false")

	// 1. Check empty repo
	commits, err := client.GetCommits(LogOptions{})
	if err != nil {
		t.Fatalf("GetCommits failed on empty repo: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("expected 0 commits, got %d", len(commits))
	}

	// 2. Create first commit
	testFile := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(testFile, []byte("hello world\nline 2\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if _, err := client.Run("add", "file1.txt"); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if _, err := client.Run("commit", "-m", "Initial commit\n\nFirst line of body"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// 3. Create second commit
	if err := os.WriteFile(testFile, []byte("hello world modified\nline 2\n"), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}
	if _, err := client.Run("commit", "-am", "Update file1"); err != nil {
		t.Fatalf("git commit 2 failed: %v", err)
	}

	// 4. Test GetCommits
	commits, err = client.GetCommits(LogOptions{Limit: 10})
	if err != nil {
		t.Fatalf("GetCommits failed: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	if commits[0].Subject != "Update file1" {
		t.Errorf("expected subject 'Update file1', got '%s'", commits[0].Subject)
	}
	if commits[1].Subject != "Initial commit" {
		t.Errorf("expected subject 'Initial commit', got '%s'", commits[1].Subject)
	}
	if commits[1].Body != "First line of body" {
		t.Errorf("expected body 'First line of body', got '%s'", commits[1].Body)
	}

	// 5. Test search filter
	commits, err = client.GetCommits(LogOptions{Search: "Update"})
	if err != nil {
		t.Fatalf("GetCommits with search failed: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit matching search, got %d", len(commits))
	}

	// 6. Test GetCommit and GetCommitDiff
	commit, diffs, err := client.GetCommit(commits[0].Hash)
	if err != nil {
		t.Fatalf("GetCommit failed: %v", err)
	}
	if commit.Subject != "Update file1" {
		t.Errorf("expected subject 'Update file1', got '%s'", commit.Subject)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 file diff, got %d", len(diffs))
	}
	if diffs[0].NewPath != "file1.txt" {
		t.Errorf("expected file path 'file1.txt', got '%s'", diffs[0].NewPath)
	}
}
