package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageAllSkipsUntracked(t *testing.T) {
	tempDir := t.TempDir()
	client := NewClient(tempDir).WithDisableSigning(true)
	if _, err := client.Run("init", "-b", "main"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	_, _ = client.Run("config", "user.name", "Test User")
	_, _ = client.Run("config", "user.email", "test@example.com")

	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(tempDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tracked.txt", "one")
	write("doomed.txt", "bye")
	if _, err := client.Run("add", "."); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if _, err := client.Run("commit", "-m", "init"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	write("tracked.txt", "two")
	os.Remove(filepath.Join(tempDir, "doomed.txt"))
	write("new.txt", "secret")

	if err := client.StageAll(); err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}

	out, err := client.Run("diff", "--cached", "--name-only")
	if err != nil {
		t.Fatalf("git diff failed: %v", err)
	}
	if got, want := out, "doomed.txt\ntracked.txt"; strings.TrimSpace(got) != want {
		t.Fatalf("staged files: got %q, want %q", got, want)
	}
}

func TestUntrackedDirsCollapseIgnoreAndDiscard(t *testing.T) {
	tempDir := t.TempDir()
	client := NewClient(tempDir).WithDisableSigning(true)
	if _, err := client.Run("init", "-b", "main"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	write := func(name string) {
		full := filepath.Join(tempDir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < untrackedDirExpandMax+5; i++ {
		write(filepath.Join("big", "nested", fmt.Sprintf("f%d.txt", i)))
	}
	write("small/a.txt")
	write("small/b.txt")
	write("loose.txt")

	paths := func() (map[string]int, int) {
		status, err := client.Status()
		if err != nil {
			t.Fatalf("Status failed: %v", err)
		}
		got := map[string]int{}
		for _, f := range status.Files {
			got[f.Path] = f.FileCount
		}
		return got, status.UntrackedCount
	}

	got, count := paths()
	want := map[string]int{"big/": untrackedDirExpandMax + 5, "small/a.txt": 0, "small/b.txt": 0, "loose.txt": 0}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("files: got %v, want %v", got, want)
	}
	if count != untrackedDirExpandMax+8 {
		t.Fatalf("untracked count: got %d, want %d", count, untrackedDirExpandMax+8)
	}

	if err := client.IgnorePath("big/"); err != nil {
		t.Fatalf("IgnorePath failed: %v", err)
	}
	if err := client.IgnorePath("../escape"); err == nil {
		t.Fatalf("IgnorePath accepted a path outside the repo")
	}
	got, _ = paths()
	if _, ok := got["big/"]; ok {
		t.Fatalf("big/ still listed after ignoring: %v", got)
	}

	// A collapsed directory is discarded by its "dir/" path.
	for i := 0; i < untrackedDirExpandMax+1; i++ {
		write(filepath.Join("gen", fmt.Sprintf("g%d.txt", i)))
	}
	if err := client.DiscardFiles([]string{"gen/"}); err != nil {
		t.Fatalf("DiscardFiles on collapsed dir failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "gen")); !os.IsNotExist(err) {
		t.Fatalf("gen/ not deleted: %v", err)
	}
}

func TestDiscardAllKeepsUntracked(t *testing.T) {
	tempDir := t.TempDir()
	client := NewClient(tempDir).WithDisableSigning(true)
	if _, err := client.Run("init", "-b", "main"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	_, _ = client.Run("config", "user.name", "Test User")
	_, _ = client.Run("config", "user.email", "test@example.com")

	tracked := filepath.Join(tempDir, "tracked.txt")
	untracked := filepath.Join(tempDir, "new", "keep.txt")
	if err := os.WriteFile(tracked, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Run("add", "."); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if _, err := client.Run("commit", "-m", "init"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}
	if err := os.WriteFile(tracked, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(untracked), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(untracked, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := client.DiscardAll(); err != nil {
		t.Fatalf("DiscardAll failed: %v", err)
	}
	if got, _ := os.ReadFile(tracked); string(got) != "one" {
		t.Fatalf("tracked file not reverted: %q", got)
	}
	if _, err := os.Stat(untracked); err != nil {
		t.Fatalf("untracked file was deleted: %v", err)
	}
}
