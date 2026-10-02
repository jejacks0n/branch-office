package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type PRAuthor struct {
	Login string `json:"login"`
}

type PRDetails struct {
	Number      int      `json:"number"`
	Title       string   `json:"title"`
	State       string   `json:"state"`
	URL         string   `json:"url"`
	Body        string   `json:"body"`
	Author      PRAuthor `json:"author"`
	HeadRefName string   `json:"headRefName"`
	BaseRefName string   `json:"baseRefName"`
}

type PRStatus struct {
	Installed bool       `json:"installed"`
	Exists    bool       `json:"exists"`
	PR        *PRDetails `json:"pr,omitempty"`
	Message   string     `json:"message,omitempty"`
}

type Client struct {
	RepoDir string
}

func NewClient(repoDir string) *Client {
	return &Client{RepoDir: repoDir}
}

func IsGHInstalled() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func (c *Client) GetPRStatus() (*PRStatus, error) {
	if !IsGHInstalled() {
		return &PRStatus{
			Installed: false,
			Exists:    false,
			Message:   "GitHub CLI (gh) is not installed",
		}, nil
	}

	cmd := exec.Command("gh", "pr", "view", "--json", "number,title,state,url,body,author,headRefName,baseRefName")
	cmd.Dir = c.RepoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// If error is "no pull requests found for branch" or similar, exists is false
		errMsg := strings.TrimSpace(stderr.String())
		return &PRStatus{
			Installed: true,
			Exists:    false,
			Message:   errMsg,
		}, nil
	}

	var pr PRDetails
	if err := json.Unmarshal(stdout.Bytes(), &pr); err != nil {
		return nil, errors.New("failed to parse gh output: " + err.Error())
	}

	return &PRStatus{
		Installed: true,
		Exists:    true,
		PR:        &pr,
	}, nil
}

// CreatePR opens a pull request for the current branch. An empty base leaves
// the target to gh, which uses the repository's default branch.
func (c *Client) CreatePR(title string, body string, base string, draft bool) (*PRDetails, error) {
	if !IsGHInstalled() {
		return nil, errors.New("gh is not installed on host machine")
	}

	args := []string{"pr", "create", "--title", title, "--body", body}
	if base != "" {
		// "=" form so a value starting with "-" can't be read as a flag.
		args = append(args, "--base="+base)
	}
	if draft {
		args = append(args, "--draft")
	}

	cmd := exec.Command("gh", args...)
	cmd.Dir = c.RepoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = strings.TrimSpace(stdout.String())
		}
		return nil, errors.New(errStr)
	}

	// Fetch full details now
	status, err := c.GetPRStatus()
	if err == nil && status.Exists && status.PR != nil {
		return status.PR, nil
	}

	createdURL := strings.TrimSpace(stdout.String())
	return &PRDetails{
		Title: title,
		URL:   createdURL,
		Body:  body,
	}, nil
}

func (c *Client) GenerateCommitMessage(ctx context.Context, diff string, hint string) (string, error) {
	if !IsGHInstalled() {
		return "", errors.New("GitHub CLI (gh) is not installed on host")
	}

	if strings.TrimSpace(diff) == "" {
		return "", errors.New("no staged changes to generate commit message from")
	}

	prompt := "Generate a concise conventional git commit message based on the diff provided via stdin.\n" +
		"STRICT INSTRUCTIONS:\n" +
		"- Wrap the final commit message in <commit_message> and </commit_message> tags, each on its own line. Anything outside the tags is discarded.\n" +
		"- Absolutely NO conversational filler, preface, commentary, or pleasantries (do NOT say 'I have enough context', 'Here is the commit message', etc.).\n" +
		"- Start directly on line 1 with the commit subject line (e.g. feat(scope): description).\n" +
		"- Follow with an optional blank line and concise bullet points.\n" +
		"- Do NOT wrap in markdown code fences or quotes.\n" +
		"- Do NOT include any Co-authored-by trailers, signatures, or co-author attribution lines."
	if strings.TrimSpace(hint) != "" {
		prompt += fmt.Sprintf("\nAuthor guidance: %s", strings.TrimSpace(hint))
	}

	cmd := exec.CommandContext(ctx, "gh", "copilot", "--", "-s", "-p", prompt, "--no-ask-user")
	cmd.Dir = c.RepoDir
	cmd.Stdin = strings.NewReader(diff)

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
		return "", errors.New("copilot error: " + errStr)
	}

	return CleanCommitMessage(stdout.String()), nil
}

// CleanCommitMessage sanitizes output from LLMs by extracting the
// <commit_message> block when present, then stripping conversational filler,
// code fences, and markdown wrappers.
func CleanCommitMessage(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}

	// Prefer the tagged block the prompt asks for. Copilot can narrate before
	// answering, so take the last block and drop everything around it.
	if start := strings.LastIndex(text, "<commit_message>"); start >= 0 {
		inner := text[start+len("<commit_message>"):]
		if end := strings.Index(inner, "</commit_message>"); end >= 0 {
			inner = inner[:end]
		}
		text = strings.TrimSpace(inner)
	}

	// Strip outer markdown code fences if present
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 2 {
			if strings.HasPrefix(lines[len(lines)-1], "```") {
				lines = lines[1 : len(lines)-1]
			} else {
				lines = lines[1:]
			}
			text = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}

	// Strip outer quotation marks if the whole message was wrapped in quotes
	if strings.HasPrefix(text, "\"") && strings.HasSuffix(text, "\"") && len(text) > 1 {
		text = strings.TrimSpace(text[1 : len(text)-1])
	}

	// Filter out common LLM preamble/conversational lines before the actual commit message
	preamblePrefixes := []string{
		"i have enough context",
		"i've got enough context",
		"here is the commit message",
		"here's the commit message",
		"here is a commit message",
		"here's a commit message",
		"here is a suggested",
		"here's a suggested",
		"here is a concise",
		"here's a concise",
		"sure, here",
		"sure! here",
		"certainly",
		"based on the diff",
		"based on the provided",
		"based on these changes",
		"commit message:",
		"proposed commit message:",
		"suggested commit message:",
	}

	lines := strings.Split(text, "\n")
	var filtered []string
	droppingPreamble := true

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isCoauthorLine(trimmed) {
			continue
		}
		if droppingPreamble {
			lower := strings.ToLower(trimmed)
			isFiller := false
			for _, prefix := range preamblePrefixes {
				if strings.HasPrefix(lower, prefix) {
					isFiller = true
					break
				}
			}
			if isFiller || trimmed == "" {
				continue
			}
			// Found the first actual content line
			droppingPreamble = false
		}
		filtered = append(filtered, line)
	}

	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

// isCoauthorLine checks if a line represents a co-author trailer or attribution.
func isCoauthorLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	clean := strings.TrimLeft(trimmed, "-* \t")
	lower := strings.ToLower(clean)
	return strings.HasPrefix(lower, "co-authored-by:") ||
		strings.HasPrefix(lower, "co-authored-by :") ||
		strings.HasPrefix(lower, "co-authored by:") ||
		strings.HasPrefix(lower, "co-authored by ") ||
		strings.HasPrefix(lower, "co-authored-by ") ||
		strings.HasPrefix(lower, "co-author:") ||
		strings.HasPrefix(lower, "co-author :") ||
		strings.HasPrefix(lower, "coauthored-by:") ||
		strings.HasPrefix(lower, "coauthor:")
}
