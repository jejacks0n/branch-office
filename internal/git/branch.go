package git

import (
	"bufio"
	"errors"
	"strings"
)

type Branch struct {
	Name       string `json:"name"`
	IsCurrent  bool   `json:"isCurrent"`
	IsRemote   bool   `json:"isRemote"`
	Upstream   string `json:"upstream,omitempty"`
	CommitHash string `json:"commitHash,omitempty"`
	CommitMsg  string `json:"commitMsg,omitempty"`
}

// ListBranches returns all local and remote branches in the repository.
func (c *Client) ListBranches() ([]Branch, error) {
	if !c.HasHead() {
		current := c.GetCurrentBranch()
		if current != "" {
			return []Branch{
				{
					Name:      current,
					IsCurrent: true,
				},
			}, nil
		}
		return []Branch{}, nil
	}

	out, err := c.Run("branch", "--all", "--format=%(refname)%00%(refname:short)%00%(HEAD)%00%(upstream:short)%00%(objectname:short)%00%(contents:subject)")
	if err != nil {
		return nil, err
	}

	return ParseBranchList(out), nil
}

// ParseBranchList parses the formatted git branch output into Branch structs.
func ParseBranchList(raw string) []Branch {
	branches := []Branch{}
	if strings.TrimSpace(raw) == "" {
		return branches
	}

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, "\x00")
		if len(parts) < 6 {
			continue
		}

		refname := parts[0]
		shortName := parts[1]
		headMarker := parts[2]
		upstream := parts[3]
		commitHash := parts[4]
		commitMsg := parts[5]

		// Skip symbolic remote HEAD references (e.g. origin/HEAD)
		if strings.HasSuffix(refname, "/HEAD") {
			continue
		}

		// Skip detached HEAD indicators in branch list
		if strings.Contains(refname, "(no branch)") || strings.Contains(refname, "(HEAD detached") {
			continue
		}

		isRemote := strings.HasPrefix(refname, "refs/remotes/")
		isCurrent := headMarker == "*"

		// Clean up branch name for display
		name := shortName
		if !isRemote {
			name = strings.TrimPrefix(refname, "refs/heads/")
		}

		branches = append(branches, Branch{
			Name:       name,
			IsCurrent:  isCurrent,
			IsRemote:   isRemote,
			Upstream:   upstream,
			CommitHash: commitHash,
			CommitMsg:  commitMsg,
		})
	}

	return branches
}

// CheckoutBranch switches to the specified branch.
// If the branch is a remote branch (e.g. origin/feature), it sets up a local tracking branch.
func (c *Client) CheckoutBranch(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return errors.New("branch name cannot be empty")
	}

	// Check if this branch is remote (e.g. "origin/foo")
	parts := strings.SplitN(branch, "/", 2)
	if len(parts) == 2 {
		remoteName := parts[0]
		localName := parts[1]

		remotesOut, err := c.Run("remote")
		if err == nil {
			remotes := strings.Fields(remotesOut)
			isRemote := false
			for _, r := range remotes {
				if r == remoteName {
					isRemote = true
					break
				}
			}

			if isRemote {
				// If local branch already exists, switch to it
				if _, err := c.Run("checkout", localName); err == nil {
					return nil
				}
				// Otherwise, checkout with tracking
				if _, err := c.Run("checkout", "--track", branch); err == nil {
					return nil
				}
			}
		}
	}

	_, err := c.Run("checkout", branch)
	return err
}

// CreateBranch creates a new branch from startPoint (or HEAD if startPoint is empty)
// and switches to it.
func (c *Client) CreateBranch(name string, startPoint string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("branch name cannot be empty")
	}

	args := []string{"checkout", "-b", name}
	startPoint = strings.TrimSpace(startPoint)
	if startPoint != "" {
		args = append(args, startPoint)
	}

	_, err := c.Run(args...)
	return err
}
