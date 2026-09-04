package tooling

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func AddDetachedWorktree(repoRoot, ref string) (string, func(), error) {
	base := filepath.Join(repoRoot, ".work")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", nil, err
	}
	path, err := os.MkdirTemp(base, "render-")
	if err != nil {
		return "", nil, err
	}
	if _, err := Run(repoRoot, "git", "worktree", "add", "--detach", path, ref); err != nil {
		_ = os.RemoveAll(path)
		return "", nil, err
	}
	cleanup := func() {
		_, _ = Run(repoRoot, "git", "worktree", "remove", "--force", path)
		_ = os.RemoveAll(path)
	}
	return path, cleanup, nil
}

func ResolveRef(repoRoot, ref string) (string, error) {
	out, err := Run(repoRoot, "git", "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func CurrentShortSHA(repoRoot string) (string, error) {
	out, err := Run(repoRoot, "git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func CommitAll(repoRoot, msg string) error {
	if _, err := Run(repoRoot, "git", "add", "-A"); err != nil {
		return err
	}
	_, err := Run(repoRoot, "git", "commit", "-m", msg)
	if err != nil {
		if strings.Contains(err.Error(), "nothing to commit") {
			return nil
		}
		return err
	}
	return nil
}

func ForcePushBranch(repoRoot, remote, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch is required")
	}
	_, err := Run(repoRoot, "git", "push", remote, "HEAD:refs/heads/"+branch, "--force-with-lease")
	return err
}

func PushTag(repoRoot, remote, tag string) error {
	if tag == "" {
		return nil
	}
	if _, err := Run(repoRoot, "git", "tag", tag); err != nil {
		return err
	}
	_, err := Run(repoRoot, "git", "push", remote, "refs/tags/"+tag)
	return err
}
