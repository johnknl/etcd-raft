package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/johnknl/rewriter/internal/rewrite"
	"github.com/johnknl/rewriter/internal/tooling"
)

func newPrivatizeCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "private-check",
		Short: "Privatize pb fields and run tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := tooling.EnsureRepoRoot(".")
			if err != nil {
				return err
			}
			if _, err := rewrite.RewritePBFieldAccess(repoRoot); err != nil {
				return err
			}
			all, err := tooling.GoFiles(repoRoot)
			if err != nil {
				return err
			}
			paths := make([]string, 0, len(all))
			for _, f := range all {
				paths = append(paths, filepath.Join(repoRoot, f))
			}
			if _, err := rewrite.RewriteFallbackPrivatePrep(paths); err != nil {
				return err
			}
			if err := tooling.PrivatizePBFields(repoRoot); err != nil {
				return err
			}
			if _, err := tooling.Run(repoRoot, "go", "test", "./..."); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "private-field validation PASS")
			return nil
		},
	}
	return cmd
}
