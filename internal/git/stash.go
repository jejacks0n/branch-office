package git

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type StashItem struct {
	Index   int    `json:"index"`
	Ref     string `json:"ref"`
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Branch  string `json:"branch"`
	Date    string `json:"date"`
}

// ListStashes lists all stash entries in the repository.
func (c *Client) ListStashes() ([]StashItem, error) {
	if !c.HasHead() {
		return []StashItem{}, nil
	}

	out, err := c.Run("stash", "list", "--format=%gd%x00%h%x00%gs%x00%cr")
	if err != nil {
		return []StashItem{}, nil
	}

	return ParseStashList(out), nil
}

// ParseStashList parses formatted git stash list output into StashItem structs.
func ParseStashList(raw string) []StashItem {
	stashes := []StashItem{}
	if strings.TrimSpace(raw) == "" {
		return stashes
	}

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, "\x00")
		if len(parts) < 4 {
			continue
		}

		ref := parts[0]
		hash := parts[1]
		subject := parts[2]
		date := parts[3]

		index := 0
		if strings.HasPrefix(ref, "stash@{") && strings.HasSuffix(ref, "}") {
			idxStr := strings.TrimSuffix(strings.TrimPrefix(ref, "stash@{"), "}")
			if parsed, err := strconv.Atoi(idxStr); err == nil {
				index = parsed
			}
		}

		branch := ""
		message := subject
		if strings.HasPrefix(subject, "WIP on ") {
			rest := strings.TrimPrefix(subject, "WIP on ")
			subParts := strings.SplitN(rest, ":", 2)
			branch = strings.TrimSpace(subParts[0])
			if len(subParts) > 1 {
				message = strings.TrimSpace(subParts[1])
			}
		} else if strings.HasPrefix(subject, "On ") {
			rest := strings.TrimPrefix(subject, "On ")
			subParts := strings.SplitN(rest, ":", 2)
			branch = strings.TrimSpace(subParts[0])
			if len(subParts) > 1 {
				message = strings.TrimSpace(subParts[1])
			}
		}

		stashes = append(stashes, StashItem{
			Index:   index,
			Ref:     ref,
			Hash:    hash,
			Message: message,
			Branch:  branch,
			Date:    date,
		})
	}

	return stashes
}

// GetStashCount returns the total number of stashes.
func (c *Client) GetStashCount() int {
	out, err := c.Run("rev-list", "--walk-reflogs", "--count", "refs/stash")
	if err != nil {
		return 0
	}
	count, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0
	}
	return count
}

// SaveStash stashes changes in the working directory and index.
func (c *Client) SaveStash(message string, includeUntracked bool) error {
	args := []string{"stash", "push"}
	if includeUntracked {
		args = append(args, "-u")
	}
	msg := strings.TrimSpace(message)
	if msg != "" {
		args = append(args, "-m", msg)
	}

	out, err := c.Run(args...)
	if err != nil {
		return err
	}
	if strings.Contains(out, "No local changes to save") {
		return errors.New("no local changes to stash")
	}
	return nil
}

// PopStash applies and drops the stash at the given index.
func (c *Client) PopStash(index int) error {
	ref := fmt.Sprintf("stash@{%d}", index)
	_, err := c.Run("stash", "pop", ref)
	return err
}

// ApplyStash applies the stash at the given index without dropping it.
func (c *Client) ApplyStash(index int) error {
	ref := fmt.Sprintf("stash@{%d}", index)
	_, err := c.Run("stash", "apply", ref)
	return err
}

// DropStash deletes the stash at the given index.
func (c *Client) DropStash(index int) error {
	ref := fmt.Sprintf("stash@{%d}", index)
	_, err := c.Run("stash", "drop", ref)
	return err
}

// ClearStashes deletes all stash entries.
func (c *Client) ClearStashes() error {
	_, err := c.Run("stash", "clear")
	return err
}

// GetStashDiff returns the git diff for the stash at the given index.
func (c *Client) GetStashDiff(index int) (string, error) {
	ref := fmt.Sprintf("stash@{%d}", index)
	out, err := c.Run("stash", "show", "-p", "-u", ref)
	if err != nil {
		// Fallback without -u if unsupported
		out, err = c.Run("stash", "show", "-p", ref)
	}
	return out, err
}
