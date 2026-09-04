package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/johnknl/rewriter/internal/tooling"
)

func newRunCmd() *cobra.Command {
	var sourceRef string
	var modulePath string
	var outputBranch string
	var pushRemote string
	var publish bool
	var tag string
	var maxIters int
	var skipPrivateCheck bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Render rewritten raft from an upstream ref",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := tooling.EnsureRepoRoot(".")
			if err != nil {
				return err
			}
			upstreamSHA, err := tooling.ResolveRef(repoRoot, sourceRef)
			if err != nil {
				return err
			}
			renderPath, cleanup, err := tooling.AddDetachedWorktree(repoRoot, sourceRef)
			if err != nil {
				return err
			}
			defer cleanup()

			_, err = runRewritePipeline(renderPath, modulePath, maxIters, func(s string, a ...interface{}) {
				fmt.Fprintf(cmd.OutOrStdout(), s+"\n", a...)
			})
			if err != nil {
				return err
			}

			rewriterSHA, err := tooling.CurrentShortSHA(repoRoot)
			if err != nil {
				return err
			}
			msg := fmt.Sprintf("rewrite: upstream %s via rewriter %s", upstreamSHA[:12], rewriterSHA)
			if err := tooling.CommitAll(renderPath, msg); err != nil {
				return err
			}

			if !skipPrivateCheck {
				renderSHA, err := tooling.ResolveRef(renderPath, "HEAD")
				if err != nil {
					return err
				}
				validatePath, validateCleanup, err := tooling.AddDetachedWorktree(repoRoot, renderSHA)
				if err != nil {
					return err
				}
				defer validateCleanup()
				if err := runPrivateValidation(validatePath, func(s string, a ...interface{}) {
					fmt.Fprintf(cmd.OutOrStdout(), s+"\n", a...)
				}); err != nil {
					return err
				}
			}
			if publish {
				if err := tooling.ForcePushBranch(renderPath, pushRemote, outputBranch); err != nil {
					return err
				}
				if tag != "" {
					if err := tooling.PushTag(renderPath, pushRemote, tag); err != nil {
						return err
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "published branch %s to %s\n", outputBranch, pushRemote)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&sourceRef, "source-ref", "upstream/main", "upstream git ref to rewrite")
	cmd.Flags().StringVar(&modulePath, "module-path", "github.com/johnknl/etcd-raft/v3", "target go module path")
	cmd.Flags().StringVar(&outputBranch, "output-branch", "etcd-main", "target branch for published artifact")
	cmd.Flags().StringVar(&pushRemote, "push-remote", "origin", "remote for branch/tag publish")
	cmd.Flags().BoolVar(&publish, "publish", false, "push output branch and optional tag")
	cmd.Flags().StringVar(&tag, "tag", "", "optional tag to create/push after publish")
	cmd.Flags().IntVar(&maxIters, "max-iters", 5, "max rewrite iterations")
	cmd.Flags().BoolVar(&skipPrivateCheck, "skip-private-check", false, "skip final private-field validation")
	return cmd
}
