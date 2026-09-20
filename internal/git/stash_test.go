package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseStashList(t *testing.T) {
	fixture := "stash@{0}\x00abc1234\x00On main: my custom stash message\x0010 minutes ago\n" +
		"stash@{1}\x00def5678\x00WIP on feat/login: 7890abc initial commit\x002 hours ago\n"

	stashes := ParseStashList(fixture)
	if len(stashes) != 2 {
		t.Fatalf("expected 2 stashes, got %d", len(stashes))
	}

	if stashes[0].Index != 0 || stashes[0].Ref != "stash@{0}" || stashes[0].Hash != "abc1234" ||
		stashes[0].Message != "my custom stash message" || stashes[0].Branch != "main" || stashes[0].Date != "10 minutes ago" {
		t.Errorf("unexpected stash[0]: %+v", stashes[0])
	}

	if stashes[1].Index != 1 || stashes[1].Ref != "stash@{1}" || stashes[1].Hash != "def5678" ||
		stashes[1].Message != "7890abc initial commit" || stashes[1].Branch != "feat/login" || stashes[1].Date != "2 hours ago" {
		t.Errorf("unexpected stash[1]: %+v", stashes[1])
	}
}

func TestStashOperations(t *testing.T) {
	tempDir := t.TempDir()
	client := NewClient(tempDir).WithDisableSigning(true)

	// In non-git or empty repo
	stashes, err := client.ListStashes()
	if err != nil {
		t.Fatalf("ListStashes failed on non-git dir: %v", err)
	}
	if len(stashes) != 0 {
		t.Fatalf("expected 0 stashes, got %d", len(stashes))
	}

	// Initialize repo
	_, _ = client.Run("init", "-b", "main")
	_, _ = client.Run("config", "user.name", "Test User")
	_, _ = client.Run("config", "user.email", "test@example.com")
	_, _ = client.Run("config", "commit.gpgSign", "false")

	// Commit an initial file
	testFile := filepath.Join(tempDir, "file.txt")
	_ = os.WriteFile(testFile, []byte("initial content\n"), 0644)
	_, _ = client.Run("add", "file.txt")
	_, err = client.Run("commit", "-m", "initial commit")
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// Initial stash count should be 0
	if client.GetStashCount() != 0 {
		t.Fatalf("expected 0 stash count, got %d", client.GetStashCount())
	}

	// Stash with no changes should return error
	if err := client.SaveStash("nothing", true); err == nil {
		t.Fatalf("expected error when stashing with no changes")
	}

	// Modify file and add untracked file
	_ = os.WriteFile(testFile, []byte("modified content\n"), 0644)
	untrackedFile := filepath.Join(tempDir, "untracked.txt")
	_ = os.WriteFile(untrackedFile, []byte("untracked content\n"), 0644)

	// Save stash with untracked files
	err = client.SaveStash("WIP feature test", true)
	if err != nil {
		t.Fatalf("SaveStash failed: %v", err)
	}

	// Check working directory is clean
	if client.GetStashCount() != 1 {
		t.Fatalf("expected 1 stash, got %d", client.GetStashCount())
	}
	stashes, err = client.ListStashes()
	if err != nil || len(stashes) != 1 {
		t.Fatalf("expected 1 stash in list, got %v (err: %v)", len(stashes), err)
	}
	if stashes[0].Message != "WIP feature test" {
		t.Fatalf("expected message 'WIP feature test', got '%s'", stashes[0].Message)
	}

	// Verify diff shows untracked file
	diff, err := client.GetStashDiff(0)
	if err != nil {
		t.Fatalf("GetStashDiff failed: %v", err)
	}
	if len(diff) == 0 {
		t.Fatalf("expected non-empty stash diff")
	}

	// Apply stash without dropping
	err = client.ApplyStash(0)
	if err != nil {
		t.Fatalf("ApplyStash failed: %v", err)
	}
	if client.GetStashCount() != 1 {
		t.Fatalf("expected stash to remain after apply, got count %d", client.GetStashCount())
	}

	// Reset working tree
	_, _ = client.Run("checkout", ".")
	_, _ = client.Run("clean", "-fd")

	// Pop stash
	err = client.PopStash(0)
	if err != nil {
		t.Fatalf("PopStash failed: %v", err)
	}
	if client.GetStashCount() != 0 {
		t.Fatalf("expected 0 stashes after pop, got %d", client.GetStashCount())
	}

	// Verify modified content and untracked file restored
	content, _ := os.ReadFile(testFile)
	if string(content) != "modified content\n" {
		t.Fatalf("expected modified content restored, got '%s'", string(content))
	}
	untrackedContent, _ := os.ReadFile(untrackedFile)
	if string(untrackedContent) != "untracked content\n" {
		t.Fatalf("expected untracked content restored, got '%s'", string(untrackedContent))
	}

	// Save another stash to test Drop and Clear
	_ = client.SaveStash("to be dropped", true)
	if client.GetStashCount() != 1 {
		t.Fatalf("expected 1 stash, got %d", client.GetStashCount())
	}
	err = client.DropStash(0)
	if err != nil {
		t.Fatalf("DropStash failed: %v", err)
	}
	if client.GetStashCount() != 0 {
		t.Fatalf("expected 0 stashes after drop, got %d", client.GetStashCount())
	}
}
