package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Client struct {
	RepoDir        string
	SSHKey         string
	DisableSigning bool
}

func NewClient(repoDir string) *Client {
	return &Client{RepoDir: repoDir}
}

func (c *Client) WithSSHKey(key string) *Client {
	c.SSHKey = key
	return c
}

func (c *Client) WithDisableSigning(disable bool) *Client {
	c.DisableSigning = disable
	return c
}

func (c *Client) Run(args ...string) (string, error) {
	return c.RunWithStdin("", args...)
}

func cleanKeyPath(p string) string {
	clean := filepath.Clean(p)
	if strings.HasPrefix(clean, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			clean = filepath.Join(home, strings.TrimPrefix(clean, "~"))
		}
	}
	return clean
}

func (c *Client) RunWithStdin(stdin string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = c.RepoDir

	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	env := os.Environ()
	var cleanEnv []string
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_OPTIONAL_LOCKS=") {
			continue
		}
		if c.SSHKey != "" && (strings.HasPrefix(e, "GIT_SSH_COMMAND=") || strings.HasPrefix(e, "SSH_AUTH_SOCK=")) {
			continue
		}
		cleanEnv = append(cleanEnv, e)
	}
	// Prevent git status / diff from writing stat caches or taking index locks
	cleanEnv = append(cleanEnv, "GIT_OPTIONAL_LOCKS=0")

	if c.SSHKey != "" {
		keyPath := cleanKeyPath(c.SSHKey)
		cleanEnv = append(cleanEnv, fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o IdentitiesOnly=yes", keyPath))
		cleanEnv = append(cleanEnv, "SSH_AUTH_SOCK=")
	}
	cmd.Env = cleanEnv

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = strings.TrimSpace(stdout.String())
		}
		if errStr == "" {
			errStr = err.Error()
		}
		return "", errors.New(errStr)
	}

	return stdout.String(), nil
}

func (c *Client) HasHead() bool {
	_, err := c.Run("rev-parse", "--verify", "HEAD")
	return err == nil
}

func (c *Client) GetCurrentBranch() string {
	out, err := c.Run("branch", "--show-current")
	if err != nil || strings.TrimSpace(out) == "" {
		// Fallback for detached head or initial commit
		rev, err2 := c.Run("rev-parse", "--abbrev-ref", "HEAD")
		if err2 == nil {
			return strings.TrimSpace(rev)
		}
		return "HEAD"
	}
	return strings.TrimSpace(out)
}

func (c *Client) GetLastCommitMessage() string {
	if !c.HasHead() {
		return ""
	}
	out, err := c.Run("log", "-1", "--format=%B")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
