package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/johnknl/rewriter/internal/tooling"
)

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check for remaining raftpb literals",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := tooling.EnsureRepoRoot(".")
			if err != nil {
				return err
			}
			files, err := tooling.GoFiles(repoRoot)
			if err != nil {
				return err
			}
			hits, err := tooling.CheckLiterals(repoRoot, files)
			if err != nil {
				return err
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "PASS: no direct raftpb composite literals found")
				return nil
			}
			for _, h := range hits {
				fmt.Fprintln(cmd.OutOrStdout(), h)
			}
			return fmt.Errorf("FAIL: direct raftpb composite literals found")
		},
	}
	return cmd
}
