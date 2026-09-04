package cli

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rewriter",
		Short: "Rewrite raftpb struct literals to wrappers",
	}
	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newCheckCmd())
	cmd.AddCommand(newPrivatizeCheckCmd())
	return cmd
}
