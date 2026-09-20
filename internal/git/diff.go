package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type DiffLine struct {
	Type    string `json:"type"` // "context", "addition", "deletion", "meta"
	OldLine int    `json:"oldLine,omitempty"`
	NewLine int    `json:"newLine,omitempty"`
	Content string `json:"content"`
}

type Hunk struct {
	Index     int        `json:"index"`
	Header    string     `json:"header"`
	OldStart  int        `json:"oldStart"`
	OldLength int        `json:"oldLength"`
	NewStart  int        `json:"newStart"`
	NewLength int        `json:"newLength"`
	Section   string     `json:"section,omitempty"`
	Lines     []DiffLine `json:"lines"`
	Patch     string     `json:"patch"`
}

type FileDiff struct {
	OldPath    string `json:"oldPath"`
	NewPath    string `json:"newPath"`
	IsNew      bool   `json:"isNew"`
	IsDeleted  bool   `json:"isDeleted"`
	IsRenamed  bool   `json:"isRenamed"`
	IsBinary   bool   `json:"isBinary"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
	Hunks      []Hunk `json:"hunks"`
	FileHeader string `json:"fileHeader"`
}

var hunkHeaderRegex = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$`)

func (c *Client) GetDiff(filePath string, staged bool, untracked bool) ([]FileDiff, error) {
	if untracked && filePath != "" {
		return c.getUntrackedDiff(filePath)
	}

	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "-U3")
	if filePath != "" {
		args = append(args, "--", filePath)
	}

	raw, err := c.Run(args...)
	if err != nil {
		return nil, err
	}

	return ParseDiff(raw), nil
}

func (c *Client) getUntrackedDiff(filePath string) ([]FileDiff, error) {
	fullPath := filepath.Join(c.RepoDir, filePath)
	if _, err := os.Stat(fullPath); err != nil {
		return nil, err
	}

	raw, err := c.Run("diff", "--no-index", "-U3", "/dev/null", filePath)
	// git diff --no-index exits with 1 when differences exist
	if err != nil && raw == "" {
		// In exec.Command, exit code 1 returns error. Let's run with command directly or fallback
		// Let's do custom execution for no-index
		raw = c.runNoIndexDiff(filePath)
	}

	return ParseDiff(raw), nil
}

func (c *Client) runNoIndexDiff(filePath string) string {
	// Custom runner that doesn't treat exit status 1 as error
	c2 := exec.Command("git", "diff", "--no-index", "-U3", "--", "/dev/null", filePath)
	c2.Dir = c.RepoDir
	out, _ := c2.Output()
	return string(out)
}

func ParseDiff(raw string) []FileDiff {
	var fileDiffs []FileDiff
	if strings.TrimSpace(raw) == "" {
		return fileDiffs
	}

	scanner := bufio.NewScanner(strings.NewReader(raw))
	var currentFile *FileDiff
	var currentHunk *Hunk
	var headerLines []string
	var hunkRawLines []string

	flushHunk := func() {
		if currentFile != nil && currentHunk != nil {
			// Construct hunk patch
			patchBuf := strings.Builder{}
			patchBuf.WriteString(currentFile.FileHeader)
			if !strings.HasSuffix(currentFile.FileHeader, "\n") {
				patchBuf.WriteString("\n")
			}
			patchBuf.WriteString(currentHunk.Header)
			patchBuf.WriteString("\n")
			for _, l := range hunkRawLines {
				patchBuf.WriteString(l)
				patchBuf.WriteString("\n")
			}
			currentHunk.Patch = patchBuf.String()
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			currentHunk = nil
			hunkRawLines = nil
		}
	}

	flushFile := func() {
		flushHunk()
		if currentFile != nil {
			fileDiffs = append(fileDiffs, *currentFile)
			currentFile = nil
			headerLines = nil
		}
	}

	oldLineNum := 0
	newLineNum := 0

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "diff --git ") {
			flushFile()
			currentFile = &FileDiff{
				Hunks: []Hunk{},
			}
			headerLines = []string{line}

			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				currentFile.OldPath = strings.TrimPrefix(parts[2], "a/")
				currentFile.NewPath = strings.TrimPrefix(parts[3], "b/")
			}
			continue
		}

		if currentFile == nil {
			continue
		}

		if currentHunk == nil {
			// Collecting file header
			if strings.HasPrefix(line, "new file mode") {
				currentFile.IsNew = true
				headerLines = append(headerLines, line)
			} else if strings.HasPrefix(line, "deleted file mode") {
				currentFile.IsDeleted = true
				headerLines = append(headerLines, line)
			} else if strings.HasPrefix(line, "similarity index") || strings.HasPrefix(line, "rename from") {
				currentFile.IsRenamed = true
				headerLines = append(headerLines, line)
			} else if strings.HasPrefix(line, "Binary files ") {
				currentFile.IsBinary = true
				headerLines = append(headerLines, line)
			} else if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "index ") {
				headerLines = append(headerLines, line)
			}
		}

		if strings.HasPrefix(line, "@@ ") {
			flushHunk()
			currentFile.FileHeader = strings.Join(headerLines, "\n")

			matches := hunkHeaderRegex.FindStringSubmatch(line)
			if len(matches) >= 4 {
				oldStart, _ := strconv.Atoi(matches[1])
				oldLen := 1
				if matches[2] != "" {
					oldLen, _ = strconv.Atoi(matches[2])
				}
				newStart, _ := strconv.Atoi(matches[3])
				newLen := 1
				if matches[4] != "" {
					newLen, _ = strconv.Atoi(matches[4])
				}
				section := strings.TrimSpace(matches[5])

				oldLineNum = oldStart
				newLineNum = newStart

				currentHunk = &Hunk{
					Index:     len(currentFile.Hunks),
					Header:    line,
					OldStart:  oldStart,
					OldLength: oldLen,
					NewStart:  newStart,
					NewLength: newLen,
					Section:   section,
					Lines:     []DiffLine{},
				}
				hunkRawLines = []string{}
			}
			continue
		}

		if currentHunk != nil {
			hunkRawLines = append(hunkRawLines, line)
			if strings.HasPrefix(line, "+") {
				currentFile.Additions++
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:    "addition",
					NewLine: newLineNum,
					Content: line[1:],
				})
				newLineNum++
			} else if strings.HasPrefix(line, "-") {
				currentFile.Deletions++
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:    "deletion",
					OldLine: oldLineNum,
					Content: line[1:],
				})
				oldLineNum++
			} else if strings.HasPrefix(line, " ") {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:    "context",
					OldLine: oldLineNum,
					NewLine: newLineNum,
					Content: line[1:],
				})
				oldLineNum++
				newLineNum++
			} else if strings.HasPrefix(line, "\\") {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:    "meta",
					Content: line,
				})
			}
		}
	}

	flushFile()
	return fileDiffs
}

func (c *Client) BuildSingleHunkPatch(filePath string, hunk Hunk) string {
	if hunk.Patch != "" {
		return hunk.Patch
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "diff --git a/%s b/%s\n", filePath, filePath)
	fmt.Fprintf(&sb, "--- a/%s\n", filePath)
	fmt.Fprintf(&sb, "+++ b/%s\n", filePath)
	sb.WriteString(hunk.Header)
	sb.WriteString("\n")
	for _, l := range hunk.Lines {
		switch l.Type {
		case "addition":
			sb.WriteString("+" + l.Content + "\n")
		case "deletion":
			sb.WriteString("-" + l.Content + "\n")
		case "context":
			sb.WriteString(" " + l.Content + "\n")
		case "meta":
			sb.WriteString(l.Content + "\n")
		}
	}
	return sb.String()
}
