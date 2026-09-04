package cli

import (
	"fmt"
	"path/filepath"

	"github.com/johnknl/rewriter/internal/rewrite"
	"github.com/johnknl/rewriter/internal/tooling"
)

type pipelineStats struct {
	LiteralChanged int
	TypedChanged   int
	GetterChanged  int
	ImportChanged  int
}

func runRewritePipeline(repoRoot, modulePath string, maxIters int, out func(string, ...interface{})) (pipelineStats, error) {
	stats := pipelineStats{}
	if err := tooling.EnsureWrappers(repoRoot); err != nil {
		return stats, err
	}

	all, err := tooling.GoFiles(repoRoot)
	if err != nil {
		return stats, err
	}
	files := make([]string, 0, len(all))
	for _, f := range all {
		if filepath.ToSlash(f) == "raftpb/wrappers.go" {
			continue
		}
		files = append(files, filepath.Join(repoRoot, f))
	}

	for i := 1; i <= maxIters; i++ {
		changed, err := rewrite.RewriteFiles(files)
		if err != nil {
			return stats, err
		}
		typedChanged, err := rewrite.RewriteTypedPBLiterals(repoRoot)
		if err != nil {
			return stats, err
		}
		getterChanged, err := rewrite.RewriteConfStateSelectorsToGetters(files)
		if err != nil {
			return stats, err
		}
		hits, err := tooling.CheckLiterals(repoRoot, all)
		if err != nil {
			return stats, err
		}
		stats.LiteralChanged += changed
		stats.TypedChanged += typedChanged
		stats.GetterChanged += getterChanged
		out("iter %d: rewritten=%d typed=%d getter_updates=%d remaining=%d", i, changed, typedChanged, getterChanged, len(hits))
		if len(hits) == 0 {
			break
		}
		if i == maxIters {
			return stats, fmt.Errorf("literals remain after %d iterations", maxIters)
		}
	}

	importChanged, err := rewrite.RewriteModulePath(repoRoot, modulePath)
	if err != nil {
		return stats, err
	}
	stats.ImportChanged = importChanged

	if _, err := tooling.Run(repoRoot, "go", "test", "./..."); err != nil {
		return stats, err
	}
	out("go test ./... PASS")
	return stats, nil
}

func runPrivateValidation(repoRoot string, out func(string, ...interface{})) error {
	all, err := tooling.GoFiles(repoRoot)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(all))
	for _, f := range all {
		paths = append(paths, filepath.Join(repoRoot, f))
	}
	fieldChanged, err := rewrite.RewritePBFieldAccess(repoRoot)
	if err != nil {
		return err
	}
	fallbackChanged, err := rewrite.RewriteFallbackPrivatePrep(paths)
	if err != nil {
		return err
	}
	out("private-check prep: field_access_updates=%d fallback_updates=%d", fieldChanged, fallbackChanged)
	if err := tooling.PrivatizePBFields(repoRoot); err != nil {
		return err
	}
	if _, err := tooling.Run(repoRoot, "go", "test", "./..."); err != nil {
		return err
	}
	out("private-field validation PASS")
	return nil
}
