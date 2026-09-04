package tooling

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const literalPattern = `&(pb|raftpb)\.(Entry|Message|HardState|ConfChange|ConfChangeSingle|ConfChangeV2|Snapshot|SnapshotMetadata|ConfState)\{`

func CheckLiterals(repoRoot string, files []string) ([]string, error) {
	args := []string{"-n", literalPattern}
	args = append(args, files...)
	cmd := exec.Command("rg", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		return splitNonEmpty(string(out)), nil
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return nil, nil
	}
	return nil, err
}

func GoFiles(repoRoot string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "*.go")
	cmd.Dir = repoRoot
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := splitNonEmpty(string(b))
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, filepath.Clean(l))
	}
	return out, nil
}

func splitNonEmpty(s string) []string {
	parts := strings.Split(strings.TrimSpace(s), "\n")
	if len(parts) == 1 && parts[0] == "" {
		return nil
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func EnsureRepoRoot(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = abs
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
