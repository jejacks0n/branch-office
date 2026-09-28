package git

import (
	"strconv"
	"strings"
)

type FileStatus struct {
	Path           string `json:"path"`
	OrigPath       string `json:"origPath,omitempty"`
	StagedStatus   string `json:"stagedStatus"`   // "none", "modified", "added", "deleted", "renamed", "typechange"
	UnstagedStatus string `json:"unstagedStatus"` // "none", "modified", "deleted", "untracked", "typechange"
	IsStaged       bool   `json:"isStaged"`
	IsUnstaged     bool   `json:"isUnstaged"`
	IsUntracked    bool   `json:"isUntracked"`
	IsConflicted   bool   `json:"isConflicted"`
	// FileCount is set on a collapsed untracked directory (Path ends in "/")
	// and holds the number of untracked files inside it.
	FileCount int `json:"fileCount,omitempty"`
}

// Untracked directories with more files than this stay collapsed to a single
// entry; smaller ones are listed file by file.
const untrackedDirExpandMax = 25

type RepoStatus struct {
	Branch            string       `json:"branch"`
	Upstream          string       `json:"upstream"`
	Ahead             int          `json:"ahead"`
	Behind            int          `json:"behind"`
	HasHead           bool         `json:"hasHead"`
	LastCommitMessage string       `json:"lastCommitMessage,omitempty"`
	Files             []FileStatus `json:"files"`
	StagedCount       int          `json:"stagedCount"`
	UnstagedCount     int          `json:"unstagedCount"`
	UntrackedCount    int          `json:"untrackedCount"`
	ConflictedCount   int          `json:"conflictedCount"`
	StashCount        int          `json:"stashCount"`
	TagCount          int          `json:"tagCount"`
}

func (c *Client) Status() (*RepoStatus, error) {
	// Default untracked mode: git reports a directory holding only untracked
	// files as one "dir/" entry instead of listing every file inside it.
	out, err := c.Run("status", "--porcelain=v2", "--branch", "-z")
	if err != nil {
		return nil, err
	}

	status := &RepoStatus{
		Branch:            c.GetCurrentBranch(),
		HasHead:           c.HasHead(),
		LastCommitMessage: c.GetLastCommitMessage(),
		StashCount:        c.GetStashCount(),
		TagCount:          c.GetTagCount(),
		Files:             []FileStatus{},
	}

	tokens := strings.Split(out, "\x00")
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token == "" {
			continue
		}

		if strings.HasPrefix(token, "# ") {
			parseBranchHeader(token, status)
			continue
		}

		if strings.HasPrefix(token, "? ") {
			path := strings.TrimPrefix(token, "? ")
			status.Files = append(status.Files, FileStatus{
				Path:           path,
				StagedStatus:   "none",
				UnstagedStatus: "untracked",
				IsUntracked:    true,
			})
			status.UntrackedCount++
			continue
		}

		if strings.HasPrefix(token, "u ") {
			parts := strings.SplitN(token, " ", 11)
			path := ""
			if len(parts) >= 11 {
				path = parts[10]
			}
			status.Files = append(status.Files, FileStatus{
				Path:           path,
				StagedStatus:   "conflicted",
				UnstagedStatus: "conflicted",
				IsConflicted:   true,
			})
			status.ConflictedCount++
			continue
		}

		if strings.HasPrefix(token, "1 ") {
			// Format: 1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
			// Split into at most 9 pieces so path with spaces remains whole
			parts := strings.SplitN(token, " ", 9)
			if len(parts) < 9 {
				continue
			}
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
			continue
		}

		if strings.HasPrefix(token, "2 ") {
			// Renamed entry: 2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <X><score> <path>
			// Followed by <origPath> in next token
			parts := strings.SplitN(token, " ", 10)
			if len(parts) < 10 {
				continue
			}
			xy := parts[1]
			path := parts[9]
			origPath := ""
			if i+1 < len(tokens) {
				origPath = tokens[i+1]
				i++ // consume origPath token
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
			continue
		}
	}

	if err := c.expandUntrackedDirs(status); err != nil {
		return nil, err
	}
	return status, nil
}

// expandUntrackedDirs counts the files inside each collapsed untracked
// directory. Small directories are replaced by their files; large ones stay
// as one entry carrying FileCount. UntrackedCount ends up counting files.
func (c *Client) expandUntrackedDirs(status *RepoStatus) error {
	dirs := map[string][]string{}
	args := []string{"ls-files", "--others", "--exclude-standard", "--full-name", "-z", "--"}
	for _, f := range status.Files {
		if f.IsUntracked && strings.HasSuffix(f.Path, "/") {
			dirs[f.Path] = nil
			args = append(args, ":(top,literal)"+f.Path)
		}
	}
	if len(dirs) == 0 {
		return nil
	}

	out, err := c.Run(args...)
	if err != nil {
		return err
	}
	for _, p := range strings.Split(out, "\x00") {
		// Git only collapses the topmost untracked directory, so exactly one
		// ancestor of each path is a key.
		for i := 0; i < len(p); i++ {
			if p[i] != '/' {
				continue
			}
			if _, ok := dirs[p[:i+1]]; ok {
				dirs[p[:i+1]] = append(dirs[p[:i+1]], p)
				break
			}
		}
	}

	files := make([]FileStatus, 0, len(status.Files))
	for _, f := range status.Files {
		inside, ok := dirs[f.Path]
		if !ok || len(inside) == 0 {
			files = append(files, f)
			continue
		}
		status.UntrackedCount += len(inside) - 1
		if len(inside) > untrackedDirExpandMax {
			f.FileCount = len(inside)
			files = append(files, f)
			continue
		}
		for _, p := range inside {
			files = append(files, FileStatus{Path: p, StagedStatus: "none", UnstagedStatus: "untracked", IsUntracked: true})
		}
	}
	status.Files = files
	return nil
}

func parseBranchHeader(token string, status *RepoStatus) {
	if strings.HasPrefix(token, "# branch.head ") {
		head := strings.TrimPrefix(token, "# branch.head ")
		if head != "(detached)" {
			status.Branch = head
		}
	} else if strings.HasPrefix(token, "# branch.upstream ") {
		status.Upstream = strings.TrimPrefix(token, "# branch.upstream ")
	} else if strings.HasPrefix(token, "# branch.ab ") {
		ab := strings.TrimPrefix(token, "# branch.ab ")
		parts := strings.Fields(ab)
		for _, p := range parts {
			if strings.HasPrefix(p, "+") {
				if n, err := strconv.Atoi(strings.TrimPrefix(p, "+")); err == nil {
					status.Ahead = n
				}
			} else if strings.HasPrefix(p, "-") {
				if n, err := strconv.Atoi(strings.TrimPrefix(p, "-")); err == nil {
					status.Behind = n
				}
			}
		}
	}
}

func parseXY(xy string) (staged string, unstaged string, isStaged bool, isUnstaged bool) {
	if len(xy) < 2 {
		return "none", "none", false, false
	}

	x := xy[0]
	y := xy[1]

	staged = mapCharToStatus(x)
	unstaged = mapCharToStatus(y)

	isStaged = x != '.'
	isUnstaged = y != '.'

	return staged, unstaged, isStaged, isUnstaged
}

func mapCharToStatus(b byte) string {
	switch b {
	case 'M':
		return "modified"
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	case 'T':
		return "typechange"
	case 'U':
		return "unmerged"
	case '.':
		return "none"
	default:
		return "unknown"
	}
}
