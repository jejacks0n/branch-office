package git

import (
	"errors"
	"strings"
)

func (c *Client) StageHunk(patch string) error {
	if strings.TrimSpace(patch) == "" {
		return errors.New("empty patch")
	}
	_, err := c.RunWithStdin(patch, "apply", "--cached", "--recount", "--whitespace=nowarn", "-")
	return err
}

func (c *Client) UnstageHunk(patch string) error {
	if strings.TrimSpace(patch) == "" {
		return errors.New("empty patch")
	}
	_, err := c.RunWithStdin(patch, "apply", "--cached", "--reverse", "--recount", "--whitespace=nowarn", "-")
	return err
}

func (c *Client) DiscardHunk(patch string) error {
	if strings.TrimSpace(patch) == "" {
		return errors.New("empty patch")
	}
	_, err := c.RunWithStdin(patch, "apply", "--reverse", "--recount", "--whitespace=nowarn", "-")
	return err
}
