package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/johnknl/rewriter/internal/rewrite"
	"github.com/johnknl/rewriter/internal/tooling"
)

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run rewrite pipeline checks without publishing",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := tooling.EnsureRepoRoot(".")
			if err != nil {
				return err
			}
			all, err := tooling.GoFiles(repoRoot)
			if err != nil {
				return err
			}
			hits, err := tooling.CheckLiterals(repoRoot, all)
			if err != nil {
				return err
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "PASS: no direct raftpb composite literals found")
			} else {
				for _, h := range hits {
					fmt.Fprintln(cmd.OutOrStdout(), h)
				}
				return fmt.Errorf("FAIL: direct raftpb composite literals found")
			}
			if _, err := rewrite.RewriteModulePath(repoRoot, "github.com/johnknl/etcd-raft/v3"); err != nil {
				return err
			}
			if _, err := tooling.Run(repoRoot, "go", "test", "./..."); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "go test ./... PASS")
			return nil
		},
	}
	return cmd
}
