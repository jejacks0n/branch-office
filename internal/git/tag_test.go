package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTagOperations(t *testing.T) {
	tempDir := t.TempDir()
	client := NewClient(tempDir).WithDisableSigning(true)

	// Non-git directory
	tags, err := client.ListTags()
	if err != nil {
		t.Fatalf("ListTags failed on non-git dir: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(tags))
	}
	if count := client.GetTagCount(); count != 0 {
		t.Fatalf("expected 0 tag count, got %d", count)
	}

	// Initialize git repo
	_, _ = client.Run("init", "-b", "main")
	_, _ = client.Run("config", "user.name", "Test User")
	_, _ = client.Run("config", "user.email", "test@example.com")
	_, _ = client.Run("config", "commit.gpgSign", "false")
	_, _ = client.Run("config", "tag.gpgSign", "false")

	// Create an initial commit
	testFile := filepath.Join(tempDir, "sample.txt")
	_ = os.WriteFile(testFile, []byte("hello world\n"), 0644)
	_, _ = client.Run("add", "sample.txt")
	_, err = client.Run("commit", "-m", "first commit")
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// Create annotated tag
	if err := client.CreateTag("v0.1.0", "Initial release", ""); err != nil {
		t.Fatalf("CreateTag annotated failed: %v", err)
	}

	// Create lightweight tag
	if err := client.CreateTag("v0.1.1-lw", "", ""); err != nil {
		t.Fatalf("CreateTag lightweight failed: %v", err)
	}

	// Count should be 2
	if count := client.GetTagCount(); count != 2 {
		t.Fatalf("expected 2 tags, got %d", count)
	}

	// List tags
	tags, err = client.ListTags()
	if err != nil {
		t.Fatalf("ListTags failed: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags in list, got %d", len(tags))
	}

	// Verify tags are listed
	tagMap := make(map[string]TagItem)
	for _, tag := range tags {
		tagMap[tag.Name] = tag
	}

	v1, ok := tagMap["v0.1.0"]
	if !ok {
		t.Fatalf("expected v0.1.0 in tags")
	}
	if !v1.IsAnnotated {
		t.Errorf("expected v0.1.0 to be annotated")
	}
	if v1.Message != "Initial release" {
		t.Errorf("expected v0.1.0 message 'Initial release', got %q", v1.Message)
	}
	if v1.CommitHash == "" {
		t.Errorf("expected non-empty commit hash for v0.1.0")
	}

	v2, ok := tagMap["v0.1.1-lw"]
	if !ok {
		t.Fatalf("expected v0.1.1-lw in tags")
	}
	if v2.IsAnnotated {
		t.Errorf("expected v0.1.1-lw to be lightweight")
	}
	if v2.CommitHash == "" {
		t.Errorf("expected non-empty commit hash for v0.1.1-lw")
	}

	// Delete one tag locally
	if err := client.DeleteTag("v0.1.1-lw", false); err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}

	if count := client.GetTagCount(); count != 1 {
		t.Fatalf("expected 1 tag after deletion, got %d", count)
	}

	// Cannot create empty tag name
	if err := client.CreateTag("", "invalid", ""); err == nil {
		t.Fatalf("expected error creating empty tag")
	}
}
