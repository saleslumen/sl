package root

import (
	"fmt"
	"io"

	"github.com/itchyny/gojq"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/rawapi"
	"github.com/spf13/cobra"
)

type Product func(*cli.Factory) *cobra.Command

func NewRootCommand(f *cli.Factory, products ...Product) *cobra.Command {
	cobra.EnableTraverseRunHooks = true
	cmd := &cobra.Command{
		Use:               "sl",
		Short:             "Saleslumen CLI",
		Long:              "Command-line interface for the public Saleslumen APIs.",
		SilenceUsage:      true,
		SilenceErrors:     true,
		DisableAutoGenTag: true,
		Args:              cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.SetVersionTemplate("sl {{.Version}}\n")
	cmd.Version = cli.Version()
	cmd.PersistentFlags().String("profile", "", "Credentials profile")
	cmd.PersistentFlags().String("namespace", "", "Namespace ID sent as sl-namespace-id")
	cmd.PersistentFlags().String("organization", "", "Organization ID used in required request bodies")
	cmd.PersistentFlags().Bool("json", false, "Print the raw JSON response")
	cmd.PersistentFlags().String("jq", "", "Filter the JSON response with a jq expression")
	cmd.PersistentFlags().Bool("verbose", false, "Log METHOD host path → status to stderr")
	cmd.PersistentFlags().Bool("yes", false, "Confirm destructive actions without prompting")
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return &cli.UsageError{Msg: err.Error()}
	})
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		return validateJQFlag(c)
	}
	cmd.SetIn(f.IO.In)
	cmd.SetOut(f.IO.Out)
	cmd.SetErr(f.IO.Err)
	cmd.AddCommand(newAuthCommand(f, cmd))
	cmd.AddCommand(newConfigCommand(f, cmd))
	cmd.AddCommand(rawapi.NewCommand(f))
	cmd.AddCommand(newVersionCommand(f))
	cmd.AddCommand(newCompletionCommand(f))
	for _, product := range products {
		cmd.AddCommand(product(f))
	}
	wrapArgumentErrors(cmd)
	return cmd
}

func wrapArgumentErrors(command *cobra.Command) {
	if command.Args != nil {
		validate := command.Args
		command.Args = func(cmd *cobra.Command, args []string) error {
			if err := validate(cmd, args); err != nil {
				return &cli.UsageError{Msg: err.Error()}
			}
			return nil
		}
	}
	for _, child := range command.Commands() {
		wrapArgumentErrors(child)
	}
}

func validateJQFlag(cmd *cobra.Command) error {
	jq, err := cmd.Flags().GetString("jq")
	if err != nil {
		return err
	}
	if jq == "" {
		return nil
	}
	if _, err := gojq.Parse(jq); err != nil {
		return &cli.UsageError{Msg: fmt.Sprintf("invalid --jq expression: %v", err)}
	}
	return nil
}

func newVersionCommand(f *cli.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print sl version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := fmt.Fprintf(f.IO.Out, "sl %s\n", cli.Version()); err != nil {
				return fmt.Errorf("write version: %w", err)
			}
			return nil
		},
	}
}

func newCompletionCommand(f *cli.Factory) *cobra.Command {
	return &cobra.Command{
		Use:                   "completion [bash|zsh|fish|powershell]",
		Short:                 "Generate shell completion scripts",
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateCompletion(cmd.Root(), f.IO.Out, args[0])
		},
	}
}

func generateCompletion(root *cobra.Command, w io.Writer, shell string) error {
	switch shell {
	case "bash":
		if err := root.GenBashCompletionV2(w, true); err != nil {
			return fmt.Errorf("generate bash completion: %w", err)
		}
		return nil
	case "zsh":
		if err := root.GenZshCompletion(w); err != nil {
			return fmt.Errorf("generate zsh completion: %w", err)
		}
		return nil
	case "fish":
		if err := root.GenFishCompletion(w, true); err != nil {
			return fmt.Errorf("generate fish completion: %w", err)
		}
		return nil
	case "powershell":
		if err := root.GenPowerShellCompletionWithDesc(w); err != nil {
			return fmt.Errorf("generate powershell completion: %w", err)
		}
		return nil
	default:
		return &cli.UsageError{Msg: fmt.Sprintf("unsupported shell %q", shell)}
	}
}
