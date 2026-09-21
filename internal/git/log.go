package git

import (
	"fmt"
	"strconv"
	"strings"
)

type CommitItem struct {
	Hash         string   `json:"hash"`
	ShortHash    string   `json:"shortHash"`
	Author       string   `json:"author"`
	Email        string   `json:"email"`
	Timestamp    int64    `json:"timestamp"`
	Date         string   `json:"date"`
	RelativeDate string   `json:"relativeDate"`
	Subject      string   `json:"subject"`
	Body         string   `json:"body"`
	Refs         []string `json:"refs"`
}

type LogOptions struct {
	Limit  int    `json:"limit"`
	Skip   int    `json:"skip"`
	Ref    string `json:"ref"`
	Path   string `json:"path"`
	Search string `json:"search"`
}

const logFormat = "%H%x1f%h%x1f%an%x1f%ae%x1f%at%x1f%ai%x1f%ar%x1f%s%x1f%b%x1f%D%x1e"

func (c *Client) GetCommits(opts LogOptions) ([]CommitItem, error) {
	if !c.HasHead() {
		return []CommitItem{}, nil
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 30
	} else if limit > 100 {
		limit = 100
	}

	args := []string{"log", fmt.Sprintf("-n%d", limit), fmt.Sprintf("--format=%s", logFormat)}
	if opts.Skip > 0 {
		args = append(args, fmt.Sprintf("--skip=%d", opts.Skip))
	}
	if opts.Search != "" {
		args = append(args, fmt.Sprintf("--grep=%s", opts.Search), "-i")
	}
	if opts.Ref != "" {
		args = append(args, opts.Ref)
	}
	if opts.Path != "" {
		args = append(args, "--", opts.Path)
	}

	out, err := c.Run(args...)
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits") || strings.Contains(out, "does not have any commits") {
			return []CommitItem{}, nil
		}
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}

	return parseCommitItems(out), nil
}

func (c *Client) GetCommit(hash string) (*CommitItem, []FileDiff, error) {
	if !c.HasHead() {
		return nil, nil, fmt.Errorf("repository has no commits")
	}

	// 1. Get commit metadata
	out, err := c.Run("log", "-n1", fmt.Sprintf("--format=%s", logFormat), hash)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get commit info for %s: %w", hash, err)
	}

	items := parseCommitItems(out)
	if len(items) == 0 {
		return nil, nil, fmt.Errorf("commit not found: %s", hash)
	}
	commit := items[0]

	// 2. Get commit diff
	diffRaw, err := c.Run("show", "--format=", "-U3", hash)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get diff for %s: %w", hash, err)
	}

	diffs := ParseDiff(diffRaw)
	return &commit, diffs, nil
}

func (c *Client) GetCommitDiff(hash string, filePath string) ([]FileDiff, error) {
	if !c.HasHead() {
		return []FileDiff{}, nil
	}

	args := []string{"show", "--format=", "-U3", hash}
	if filePath != "" {
		args = append(args, "--", filePath)
	}

	diffRaw, err := c.Run(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit diff: %w", err)
	}

	return ParseDiff(diffRaw), nil
}

func parseCommitItems(raw string) []CommitItem {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []CommitItem{}
	}

	records := strings.Split(raw, "\x1e")
	commits := make([]CommitItem, 0, len(records))

	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		parts := strings.Split(record, "\x1f")
		if len(parts) < 10 {
			continue
		}

		hash := strings.TrimSpace(parts[0])
		shortHash := strings.TrimSpace(parts[1])
		author := strings.TrimSpace(parts[2])
		email := strings.TrimSpace(parts[3])
		ts, _ := strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 64)
		date := strings.TrimSpace(parts[5])
		relativeDate := strings.TrimSpace(parts[6])
		subject := strings.TrimSpace(parts[7])
		body := strings.TrimSpace(parts[8])
		refsStr := strings.TrimSpace(parts[9])

		var refs []string
		if refsStr != "" {
			for _, r := range strings.Split(refsStr, ",") {
				r = strings.TrimSpace(r)
				if r != "" {
					refs = append(refs, r)
				}
			}
		}

		commits = append(commits, CommitItem{
			Hash:         hash,
			ShortHash:    shortHash,
			Author:       author,
			Email:        email,
			Timestamp:    ts,
			Date:         date,
			RelativeDate: relativeDate,
			Subject:      subject,
			Body:         body,
			Refs:         refs,
		})
	}

	return commits
}
