package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (c *Client) StageFiles(files []string) error {
	if len(files) == 0 {
		return errors.New("no files specified to stage")
	}
	args := append([]string{"add", "--"}, files...)
	_, err := c.Run(args...)
	return err
}

// StageAll stages every change to tracked files, deletions included. Untracked
// files are left alone so new files are never committed by accident; stage
// those explicitly.
func (c *Client) StageAll() error {
	_, err := c.Run("add", "-u")
	return err
}

func (c *Client) UnstageFiles(files []string) error {
	if len(files) == 0 {
		return errors.New("no files specified to unstage")
	}

	if !c.HasHead() {
		args := append([]string{"rm", "--cached", "-r", "--"}, files...)
		_, err := c.Run(args...)
		return err
	}

	args := append([]string{"restore", "--staged", "--"}, files...)
	_, err := c.Run(args...)
	return err
}

func (c *Client) UnstageAll() error {
	if !c.HasHead() {
		_, err := c.Run("rm", "--cached", "-r", ".")
		return err
	}
	_, err := c.Run("restore", "--staged", ".")
	return err
}

func (c *Client) DiscardFiles(files []string) error {
	if len(files) == 0 {
		return errors.New("no files specified to discard")
	}

	// We determine whether each file is untracked or tracked
	status, err := c.Status()
	if err != nil {
		return err
	}

	untrackedMap := make(map[string]bool)
	for _, f := range status.Files {
		if f.IsUntracked {
			// Clean drops the trailing slash on collapsed directories.
			untrackedMap[filepath.Clean(f.Path)] = true
		}
	}

	var tracked []string
	var untracked []string

	for _, f := range files {
		clean := filepath.Clean(f)
		if untrackedMap[clean] {
			untracked = append(untracked, clean)
		} else {
			tracked = append(tracked, clean)
		}
	}

	if len(tracked) > 0 {
		// First restore working tree
		args := append([]string{"restore", "--"}, tracked...)
		if _, err := c.Run(args...); err != nil {
			return err
		}
	}

	for _, u := range untracked {
		fullPath := filepath.Join(c.RepoDir, u)
		// Ensure path is within repo
		rel, err := filepath.Rel(c.RepoDir, fullPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return errors.New("security error: path outside repo directory")
		}
		if err := os.RemoveAll(fullPath); err != nil {
			return err
		}
	}

	return nil
}

// IgnorePath appends an entry for path, relative to the repo root, to the
// root .gitignore. Directories should end in "/". The entry is anchored with a
// leading slash so it only matches that exact path.
func (c *Client) IgnorePath(path string) error {
	clean := filepath.ToSlash(filepath.Clean(path))
	if path == "" || strings.ContainsAny(path, "\r\n") || filepath.IsAbs(path) || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return errors.New("invalid path")
	}
	if strings.HasSuffix(path, "/") {
		clean += "/"
	}
	// Escape glob and escape characters so the path matches literally.
	entry := "/" + strings.NewReplacer(`\`, `\\`, "*", `\*`, "?", `\?`, "[", `\[`).Replace(clean)

	ignorePath := filepath.Join(c.RepoDir, ".gitignore")
	existing, err := os.ReadFile(ignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		entry = "\n" + entry
	}
	f, err := os.OpenFile(ignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(entry + "\n"); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// DiscardAll reverts unstaged changes to tracked files. Untracked files are
// left alone; delete those explicitly.
func (c *Client) DiscardAll() error {
	_, err := c.Run("restore", ".")
	return err
}

func (c *Client) signingArgs() []string {
	var args []string
	if c.DisableSigning {
		args = append(args, "-c", "commit.gpgSign=false", "-c", "tag.gpgSign=false", "-c", "gpg.ssh.defaultKeyCommand=")
	} else if c.SSHKey != "" {
		keyPath := cleanKeyPath(c.SSHKey)
		pubPath := keyPath + ".pub"
		signingKey := keyPath
		if _, err := os.Stat(pubPath); err == nil {
			signingKey = pubPath
		}
		if _, err := os.Stat(keyPath); err == nil {
			args = append(args, "-c", fmt.Sprintf("user.signingKey=%s", signingKey), "-c", "gpg.ssh.defaultKeyCommand=")
		} else {
			args = append(args, "-c", "commit.gpgSign=false", "-c", "tag.gpgSign=false", "-c", "gpg.ssh.defaultKeyCommand=")
		}
	}
	return args
}

func (c *Client) Commit(message string, amend bool) error {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return errors.New("commit message cannot be empty")
	}

	args := c.signingArgs()
	args = append(args, "commit", "-m", msg)
	if amend {
		args = append(args, "--amend")
	}

	_, err := c.Run(args...)
	return err
}

func (c *Client) Push(forceWithLease bool, setUpstream bool) error {
	args := []string{"push"}
	if forceWithLease {
		args = append(args, "--force-with-lease")
	}

	if setUpstream {
		branch := c.GetCurrentBranch()
		if branch == "" || branch == "HEAD" {
			return errors.New("cannot push detached HEAD")
		}
		args = append(args, "-u", "origin", branch)
	}

	_, err := c.Run(args...)
	return err
}

func (c *Client) Pull(rebase bool, setUpstream bool) error {
	branch := c.GetCurrentBranch()
	if branch == "" || branch == "HEAD" {
		return errors.New("cannot pull detached HEAD")
	}

	if setUpstream {
		_, _ = c.Run("branch", fmt.Sprintf("--set-upstream-to=origin/%s", branch), branch)
	}

	args := c.signingArgs()
	args = append(args, "pull")
	if rebase {
		args = append(args, "--rebase")
	}

	_, err := c.Run(args...)
	return err
}

func (c *Client) Fetch() error {
	_, err := c.Run("fetch", "--prune")
	return err
}

