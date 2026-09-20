package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseBranchList(t *testing.T) {
	fixture := "refs/heads/main\x00main\x00*\x00origin/main\x00abc1234\x00feat: initial work\n" +
		"refs/heads/feat/ui\x00feat/ui\x00\x00\x00def5678\x00add ui components\n" +
		"refs/remotes/origin/main\x00origin/main\x00\x00\x00abc1234\x00feat: initial work\n" +
		"refs/remotes/origin/HEAD\x00origin/HEAD\x00\x00\x00abc1234\x00feat: initial work\n" +
		"refs/remotes/origin/feature-remote\x00origin/feature-remote\x00\x00\x00789abcd\x00remote branch\n"

	branches := ParseBranchList(fixture)
	if len(branches) != 4 {
		t.Fatalf("expected 4 branches (origin/HEAD should be filtered), got %d", len(branches))
	}

	// First branch: main (local, current)
	if branches[0].Name != "main" || !branches[0].IsCurrent || branches[0].IsRemote || branches[0].Upstream != "origin/main" || branches[0].CommitHash != "abc1234" || branches[0].CommitMsg != "feat: initial work" {
		t.Errorf("unexpected branch[0]: %+v", branches[0])
	}

	// Second branch: feat/ui (local, not current)
	if branches[1].Name != "feat/ui" || branches[1].IsCurrent || branches[1].IsRemote {
		t.Errorf("unexpected branch[1]: %+v", branches[1])
	}

	// Third branch: origin/main (remote)
	if branches[2].Name != "origin/main" || branches[2].IsCurrent || !branches[2].IsRemote {
		t.Errorf("unexpected branch[2]: %+v", branches[2])
	}

	// Fourth branch: origin/feature-remote (remote)
	if branches[3].Name != "origin/feature-remote" || branches[3].IsCurrent || !branches[3].IsRemote {
		t.Errorf("unexpected branch[3]: %+v", branches[3])
	}
}

func TestBranchOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "broffice-branch-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Initialize repo
	exec.Command("git", "init", "-b", "main", repoDir).Run()
	exec.Command("git", "-C", repoDir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", repoDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", repoDir, "config", "commit.gpgSign", "false").Run()

	client := NewClient(repoDir)

	// List on empty repo without commits
	branches, err := client.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches on empty repo failed: %v", err)
	}
	if len(branches) != 1 || branches[0].Name != "main" || !branches[0].IsCurrent {
		t.Fatalf("unexpected branches on empty repo: %+v", branches)
	}

	// Add an initial commit
	dummyFile := filepath.Join(repoDir, "file.txt")
	os.WriteFile(dummyFile, []byte("hello"), 0644)
	client.Run("add", "file.txt")
	_, err = client.Run("commit", "-m", "initial commit")
	if err != nil {
		t.Fatalf("failed initial commit: %v", err)
	}

	// List branches with initial commit
	branches, err = client.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches failed: %v", err)
	}
	if len(branches) != 1 || branches[0].Name != "main" || !branches[0].IsCurrent || branches[0].CommitMsg != "initial commit" {
		t.Fatalf("unexpected branches: %+v", branches)
	}

	// Create a new branch 'feature-1'
	err = client.CreateBranch("feature-1", "")
	if err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	if client.GetCurrentBranch() != "feature-1" {
		t.Fatalf("expected current branch to be feature-1, got %s", client.GetCurrentBranch())
	}

	// Switch back to 'main'
	err = client.CheckoutBranch("main")
	if err != nil {
		t.Fatalf("CheckoutBranch failed: %v", err)
	}
	if client.GetCurrentBranch() != "main" {
		t.Fatalf("expected current branch to be main, got %s", client.GetCurrentBranch())
	}

	// Create a branch from explicit start point
	err = client.CreateBranch("feature-2", "main")
	if err != nil {
		t.Fatalf("CreateBranch with startPoint failed: %v", err)
	}
	if client.GetCurrentBranch() != "feature-2" {
		t.Fatalf("expected current branch to be feature-2, got %s", client.GetCurrentBranch())
	}

	// Verify all 3 local branches exist
	branches, err = client.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches failed: %v", err)
	}
	if len(branches) != 3 {
		t.Fatalf("expected 3 branches, got %d", len(branches))
	}
}
