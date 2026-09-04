package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/johnknl/rewriter/internal/rewrite"
	"github.com/johnknl/rewriter/internal/tooling"
)

func newRunCmd() *cobra.Command {
	var maxIters int
	var skipReset bool
	var skipPrivateCheck bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Reset, rewrite, check, and test",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := tooling.EnsureRepoRoot(".")
			if err != nil {
				return err
			}

			const toolRel = "."
			if !skipReset {
				if err := tooling.ResetRepoToHeadExceptTool(repoRoot, toolRel); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "reset complete")
			}

			if err := tooling.EnsureWrappers(repoRoot); err != nil {
				return err
			}

			all, err := tooling.GoFiles(repoRoot)
			if err != nil {
				return err
			}
			files := make([]string, 0, len(all))
			for _, f := range all {
				s := filepath.ToSlash(f)
				if s == "raftpb/wrappers.go" {
					continue
				}
				if len(s) >= len(toolRel)+1 && s[:len(toolRel)+1] == toolRel+"/" {
					continue
				}
				files = append(files, filepath.Join(repoRoot, f))
			}

			for i := 1; i <= maxIters; i++ {
				changed, err := rewrite.RewriteFiles(files)
				if err != nil {
					return err
				}
				typedChanged, err := rewrite.RewriteTypedPBLiterals(repoRoot)
				if err != nil {
					return err
				}
				getterChanged, err := rewrite.RewriteConfStateSelectorsToGetters(files)
				if err != nil {
					return err
				}
				hits, err := tooling.CheckLiterals(repoRoot, all)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "iter %d: rewritten=%d typed=%d getter_updates=%d remaining=%d\n", i, changed, typedChanged, getterChanged, len(hits))
				if len(hits) == 0 {
					break
				}
				if i == maxIters {
					return fmt.Errorf("literals remain after %d iterations", maxIters)
				}
			}

			if _, err := tooling.Run(repoRoot, "go", "test", "./..."); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "go test ./... PASS")

			if !skipPrivateCheck {
				fieldChanged, err := rewrite.RewritePBFieldAccess(repoRoot)
				if err != nil {
					return err
				}
				fallbackChanged, err := rewrite.RewriteFallbackPrivatePrep(files)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "private-check prep: field_access_updates=%d fallback_updates=%d\n", fieldChanged, fallbackChanged)
				if err := tooling.PrivatizePBFields(repoRoot); err != nil {
					return err
				}
				if _, err := tooling.Run(repoRoot, "go", "test", "./..."); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "private-field validation PASS")
			}

			return nil
		},
	}
	cmd.Flags().IntVar(&maxIters, "max-iters", 5, "max rewrite iterations")
	cmd.Flags().BoolVar(&skipReset, "skip-reset", false, "skip reset to HEAD")
	cmd.Flags().BoolVar(&skipPrivateCheck, "skip-private-check", false, "skip final private-field validation")
	return cmd
}
