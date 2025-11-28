package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for ghcrctl.

To load completions:

Bash:
  $ source <(ghcrctl completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ ghcrctl completion bash > /etc/bash_completion.d/ghcrctl
  # macOS:
  $ ghcrctl completion bash > $(brew --prefix)/etc/bash_completion.d/ghcrctl

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  # To load completions for each session, execute once:
  $ ghcrctl completion zsh > "${fpath[1]}/_ghcrctl"
  # You will need to start a new shell for this setup to take effect.

Fish:
  $ ghcrctl completion fish | source
  # To load completions for each session, execute once:
  $ ghcrctl completion fish > ~/.config/fish/completions/ghcrctl.fish

PowerShell:
  PS> ghcrctl completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> ghcrctl completion powershell > ghcrctl.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}

	return cmd
}

// completeImageRef provides dynamic completion for owner/image[:tag] references.
// It queries the GitHub API to list packages for the specified owner.
func completeImageRef(cmd *cobra.Command, toComplete string) []string {
	// Need at least owner/ prefix to complete
	slashIdx := strings.Index(toComplete, "/")
	if slashIdx == -1 {
		return nil
	}

	owner := toComplete[:slashIdx]
	if owner == "" {
		return nil
	}

	// Get token - completions fail silently without token
	token, err := gh.GetToken()
	if err != nil {
		return nil
	}

	// Create client
	ctx := context.Background()
	if cmd != nil {
		ctx = cmd.Context()
	}
	client, err := gh.NewClientWithContext(ctx, token)
	if err != nil {
		return nil
	}

	// Determine owner type
	ownerType, err := client.GetOwnerType(ctx, owner)
	if err != nil {
		return nil
	}

	// List packages for owner
	packages, err := client.ListPackages(ctx, owner, ownerType)
	if err != nil {
		return nil
	}

	// Filter and format completions
	prefix := toComplete[slashIdx+1:]
	var completions []string
	for _, pkg := range packages {
		if strings.HasPrefix(pkg, prefix) {
			completions = append(completions, owner+"/"+pkg)
		}
	}

	return completions
}

// imageRefValidArgsFunc returns a ValidArgsFunction for commands that take owner/image[:tag]
func imageRefValidArgsFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete first argument
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	completions := completeImageRef(cmd, toComplete)
	return completions, cobra.ShellCompDirectiveNoFileComp
}
