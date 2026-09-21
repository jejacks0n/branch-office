package git

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type TagItem struct {
	Name        string `json:"name"`
	CommitHash  string `json:"commitHash"`
	Date        string `json:"date"`
	IsAnnotated bool   `json:"isAnnotated"`
	Message     string `json:"message"`
}

func (c *Client) ListTags() ([]TagItem, error) {
	if !c.HasHead() {
		return []TagItem{}, nil
	}

	out, err := c.Run("tag", "-l", "--sort=-creatordate", "--format=%(refname:strip=2)%00%(objectname:short)%00%(*objectname:short)%00%(creatordate:relative)%00%(objecttype)%00%(contents:subject)")
	if err != nil {
		return []TagItem{}, nil
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	tags := make([]TagItem, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\x00")
		if len(parts) < 6 {
			continue
		}

		name := parts[0]
		objHash := parts[1]
		derefHash := parts[2]
		date := parts[3]
		objType := parts[4]
		subject := parts[5]

		isAnnotated := objType == "tag"
		commitHash := objHash
		if isAnnotated && derefHash != "" {
			commitHash = derefHash
		}

		tags = append(tags, TagItem{
			Name:        name,
			CommitHash:  commitHash,
			Date:        date,
			IsAnnotated: isAnnotated,
			Message:     subject,
		})
	}

	return tags, nil
}

func (c *Client) GetTagCount() int {
	if !c.HasHead() {
		return 0
	}
	out, err := c.Run("tag", "-l")
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	count := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			count++
		}
	}
	return count
}

func (c *Client) CreateTag(name string, message string, targetCommit string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("tag name cannot be empty")
	}

	args := []string{}
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

	args = append(args, "tag")
	msg := strings.TrimSpace(message)
	if msg != "" {
		args = append(args, "-a", name, "-m", msg)
	} else {
		args = append(args, name)
	}

	if strings.TrimSpace(targetCommit) != "" {
		args = append(args, strings.TrimSpace(targetCommit))
	}

	_, err := c.Run(args...)
	return err
}

func (c *Client) PushTag(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("tag name cannot be empty")
	}

	_, err := c.Run("push", "origin", name)
	return err
}

func (c *Client) PushAllTags() error {
	_, err := c.Run("push", "origin", "--tags")
	return err
}

func (c *Client) DeleteTag(name string, remote bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("tag name cannot be empty")
	}

	if _, err := c.Run("tag", "-d", name); err != nil {
		return err
	}

	if remote {
		if _, err := c.Run("push", "origin", "--delete", name); err != nil {
			return err
		}
	}

	return nil
}
