package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestConfigStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "broffice-config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfgPath := filepath.Join(tempDir, "config.json")
	store, err := NewStore(cfgPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	repos := store.ListRepos()
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(repos))
	}

	// Create a dummy git repo directory
	gitDir := filepath.Join(tempDir, "sample-repo")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", gitDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git init: %v", err)
	}

	// Add the repo
	repo, err := store.AddRepo(gitDir)
	if err != nil {
		t.Fatalf("failed to add repo: %v", err)
	}

	if repo.Name != "sample-repo" {
		t.Errorf("expected repo name 'sample-repo', got '%s'", repo.Name)
	}

	repos = store.ListRepos()
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}

	// Fetch by ID
	found, err := store.GetRepo(repo.ID)
	if err != nil || found == nil {
		t.Fatalf("failed to get repo by ID: %v", err)
	}

	// Try adding same repo again (idempotent)
	repo2, err := store.AddRepo(gitDir)
	if err != nil {
		t.Fatalf("expected idempotent add, got error: %v", err)
	}
	if repo2.ID != repo.ID {
		t.Errorf("expected same ID on re-add, got %s vs %s", repo2.ID, repo.ID)
	}
	if len(store.ListRepos()) != 1 {
		t.Errorf("expected still 1 repo, got %d", len(store.ListRepos()))
	}

	// Remove repo
	if err := store.RemoveRepo(repo.ID); err != nil {
		t.Fatalf("failed to remove repo: %v", err)
	}

	if len(store.ListRepos()) != 0 {
		t.Errorf("expected 0 repos after removal, got %d", len(store.ListRepos()))
	}
}
