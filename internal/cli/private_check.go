package cli

import (
	"fmt"

	"github.com/spf13/cobra"

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
			if err := runPrivateValidation(repoRoot, func(s string, a ...interface{}) {
				fmt.Fprintf(cmd.OutOrStdout(), s+"\n", a...)
			}); err != nil {
				return err
			}
			return nil
		},
	}
	return cmd
}
