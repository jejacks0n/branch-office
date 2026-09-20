package git

import (
	"testing"
)

func TestParsePorcelainV2(t *testing.T) {
	// Sample porcelain v2 -z output
	// Notice items are separated by \x00
	raw := "# branch.oid 1234567890abcdef\x00" +
		"# branch.head feature/branch\x00" +
		"# branch.upstream origin/feature/branch\x00" +
		"# branch.ab +2 -1\x00" +
		"1 .M N... 100644 100644 100644 aaaaaaa bbbbbbb unstaged_modified.txt\x00" +
		"1 M. N... 100644 100644 100644 aaaaaaa bbbbbbb staged_modified.txt\x00" +
		"1 MM N... 100644 100644 100644 aaaaaaa bbbbbbb both_modified.txt\x00" +
		"1 A. N... 000000 100644 100644 0000000 bbbbbbb staged_new.txt\x00" +
		"1 D. N... 100644 000000 000000 aaaaaaa 0000000 staged_deleted.txt\x00" +
		"? untracked_file.go\x00" +
		"2 R. N... 100644 100644 100644 aaaaaaa bbbbbbb R100 new_name.txt\x00old_name.txt\x00"

	status := &RepoStatus{
		Files: []FileStatus{},
	}

	tokens := parseTokens(raw)
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token == "" {
			continue
		}
		if token[0] == '#' {
			parseBranchHeader(token, status)
		} else if token[0] == '?' {
			status.Files = append(status.Files, FileStatus{
				Path:        token[2:],
				IsUntracked: true,
			})
			status.UntrackedCount++
		} else if token[0] == '1' {
			parts := splitNSpaces(token, 9)
			xy := parts[1]
			path := parts[8]
			staged, unstaged, isStaged, isUnstaged := parseXY(xy)
			status.Files = append(status.Files, FileStatus{
				Path:           path,
				StagedStatus:   staged,
				UnstagedStatus: unstaged,
				IsStaged:       isStaged,
				IsUnstaged:     isUnstaged,
			})
			if isStaged {
				status.StagedCount++
			}
			if isUnstaged {
				status.UnstagedCount++
			}
		} else if token[0] == '2' {
			parts := splitNSpaces(token, 10)
			xy := parts[1]
			path := parts[9]
			origPath := ""
			if i+1 < len(tokens) {
				origPath = tokens[i+1]
				i++
			}
			staged, unstaged, isStaged, isUnstaged := parseXY(xy)
			status.Files = append(status.Files, FileStatus{
				Path:           path,
				OrigPath:       origPath,
				StagedStatus:   staged,
				UnstagedStatus: unstaged,
				IsStaged:       isStaged,
				IsUnstaged:     isUnstaged,
			})
			if isStaged {
				status.StagedCount++
			}
			if isUnstaged {
				status.UnstagedCount++
			}
		}
	}

	if status.Branch != "feature/branch" {
		t.Fatalf("expected branch 'feature/branch', got '%s'", status.Branch)
	}
	if status.Upstream != "origin/feature/branch" {
		t.Fatalf("expected upstream 'origin/feature/branch', got '%s'", status.Upstream)
	}
	if status.Ahead != 2 || status.Behind != 1 {
		t.Fatalf("expected ahead 2 behind 1, got +%d -%d", status.Ahead, status.Behind)
	}
	if status.StagedCount != 5 {
		t.Fatalf("expected 5 staged items, got %d", status.StagedCount)
	}
	if status.UnstagedCount != 2 {
		t.Fatalf("expected 2 unstaged items, got %d", status.UnstagedCount)
	}
	if status.UntrackedCount != 1 {
		t.Fatalf("expected 1 untracked item, got %d", status.UntrackedCount)
	}
}

func parseTokens(raw string) []string {
	var tokens []string
	curr := ""
	for i := 0; i < len(raw); i++ {
		if raw[i] == 0 {
			tokens = append(tokens, curr)
			curr = ""
		} else {
			curr += string(raw[i])
		}
	}
	if curr != "" {
		tokens = append(tokens, curr)
	}
	return tokens
}

func splitNSpaces(s string, n int) []string {
	var res []string
	start := 0
	for count := 1; count < n; count++ {
		idx := -1
		for i := start; i < len(s); i++ {
			if s[i] == ' ' {
				idx = i
				break
			}
		}
		if idx == -1 {
			break
		}
		res = append(res, s[start:idx])
		start = idx + 1
	}
	res = append(res, s[start:])
	return res
}

func TestGetLastCommitMessage(t *testing.T) {
	tempDir := t.TempDir()
	client := NewClient(tempDir).WithDisableSigning(true)

	// In empty non-git dir
	if client.GetLastCommitMessage() != "" {
		t.Fatalf("expected empty commit message for non-git dir")
	}

	// Init git repo
	_, err := client.Run("init", "-b", "main")
	if err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	_, _ = client.Run("config", "user.name", "Test User")
	_, _ = client.Run("config", "user.email", "test@example.com")
	_, _ = client.Run("config", "commit.gpgSign", "false")

	// Empty repo has no head
	if client.GetLastCommitMessage() != "" {
		t.Fatalf("expected empty commit message for repo without commits")
	}

	// Initial commit
	if _, err := client.Run("commit", "--allow-empty", "-m", "feat: initial commit for testing"); err != nil {
		t.Fatalf("failed commit: %v", err)
	}
	msg := client.GetLastCommitMessage()
	if msg != "feat: initial commit for testing" {
		t.Fatalf("expected 'feat: initial commit for testing', got '%s'", msg)
	}

	// Status includes last commit message
	status, err := client.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}
	if status.LastCommitMessage != "feat: initial commit for testing" {
		t.Fatalf("expected status.LastCommitMessage 'feat: initial commit for testing', got '%s'", status.LastCommitMessage)
	}
}
