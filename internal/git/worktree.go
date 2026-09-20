package git

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Worktree struct {
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	Head       string `json:"head"`
	IsMain     bool   `json:"isMain"`
	IsLocked   bool   `json:"isLocked"`
	LockReason string `json:"lockReason,omitempty"`
	Prunable   bool   `json:"prunable"`
}

func (c *Client) GetGitDir() (string, error) {
	out, err := c.Run("rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	gitDir := strings.TrimSpace(out)
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(c.RepoDir, gitDir)
	}
	return filepath.Clean(gitDir), nil
}

func (c *Client) ListWorktrees() ([]Worktree, error) {
	out, err := c.Run("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return ParseWorktreeList(out), nil
}

func ParseWorktreeList(raw string) []Worktree {
	worktrees := []Worktree{}
	if strings.TrimSpace(raw) == "" {
		return worktrees
	}

	scanner := bufio.NewScanner(strings.NewReader(raw))
	var current *Worktree

	flush := func() {
		if current != nil && current.Path != "" {
			if len(worktrees) == 0 {
				current.IsMain = true
			}
			worktrees = append(worktrees, *current)
			current = nil
		}
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			flush()
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		key := parts[0]
		val := ""
		if len(parts) > 1 {
			val = parts[1]
		}

		switch key {
		case "worktree":
			flush()
			current = &Worktree{
				Path: filepath.Clean(val),
			}
		case "HEAD":
			if current != nil {
				current.Head = val
			}
		case "branch":
			if current != nil {
				// Strip "refs/heads/"
				current.Branch = strings.TrimPrefix(val, "refs/heads/")
			}
		case "detached":
			if current != nil && current.Branch == "" {
				current.Branch = "HEAD (detached)"
			}
		case "locked":
			if current != nil {
				current.IsLocked = true
				current.LockReason = val
			}
		case "prunable":
			if current != nil {
				current.Prunable = true
			}
		}
	}

	flush()
	return worktrees
}

func (c *Client) AddWorktree(targetPath string, branch string, createBranch bool) (*Worktree, error) {
	cleanPath := targetPath
	if strings.HasPrefix(cleanPath, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			cleanPath = filepath.Join(home, strings.TrimPrefix(cleanPath, "~"))
		}
	}
	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(c.RepoDir, cleanPath)
	}
	cleanPath = filepath.Clean(cleanPath)

	args := []string{"worktree", "add"}
	if createBranch && branch != "" {
		args = append(args, "-b", branch, cleanPath)
	} else if branch != "" {
		args = append(args, cleanPath, branch)
	} else {
		args = append(args, cleanPath)
	}

	if _, err := c.Run(args...); err != nil {
		return nil, err
	}

	worktrees, err := c.ListWorktrees()
	if err != nil {
		return &Worktree{Path: cleanPath, Branch: branch}, nil
	}

	for _, wt := range worktrees {
		if wt.Path == cleanPath {
			copyWt := wt
			return &copyWt, nil
		}
	}

	return &Worktree{Path: cleanPath, Branch: branch}, nil
}

func (c *Client) RemoveWorktree(targetPath string, force bool) error {
	cleanPath := targetPath
	if strings.HasPrefix(cleanPath, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			cleanPath = filepath.Join(home, strings.TrimPrefix(cleanPath, "~"))
		}
	}
	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(c.RepoDir, cleanPath)
	}
	cleanPath = filepath.Clean(cleanPath)

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, cleanPath)

	_, err := c.Run(args...)
	if err != nil {
		// If git worktree remove failed because directory was deleted manually, run git worktree prune
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "no such file") {
			_, _ = c.Run("worktree", "prune")
			return nil
		}
		return errors.New(err.Error())
	}

	return nil
}
