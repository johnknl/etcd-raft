package tooling

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ResetRepoToHeadExceptTool(repoRoot string, toolRel string) error {
	files, err := trackedFiles(repoRoot)
	if err != nil {
		return err
	}
	var targets []string
	prefix := filepath.ToSlash(strings.TrimSuffix(toolRel, "/")) + "/"
	for _, f := range files {
		s := filepath.ToSlash(f)
		if strings.HasPrefix(s, prefix) {
			continue
		}
		targets = append(targets, s)
	}
	if len(targets) == 0 {
		return removeUntrackedOutsideTool(repoRoot, prefix)
	}
	args := []string{"checkout", "HEAD", "--"}
	args = append(args, targets...)
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return removeUntrackedOutsideTool(repoRoot, prefix)
}

func removeUntrackedOutsideTool(repoRoot, toolPrefix string) error {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = repoRoot
	b, err := cmd.Output()
	if err != nil {
		return err
	}
	for _, rel := range splitNonEmpty(string(b)) {
		s := filepath.ToSlash(filepath.Clean(rel))
		if strings.HasPrefix(s, toolPrefix) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(repoRoot, s)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func trackedFiles(repoRoot string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = repoRoot
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return splitNonEmpty(string(b)), nil
}
