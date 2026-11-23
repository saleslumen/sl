package config

import (
	"errors"
	"fmt"
	"io"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/credentials"
	"github.com/spf13/cobra"
)

var ErrNoStore = errors.New("config: store resolver is not configured")

type Streams struct {
	Out io.Writer
}

type Deps struct {
	Streams Streams
	Store   func() (credentials.Store, error)
	Profile func() string
}

func NewCommand(d Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage profile configuration", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newGetCommand(d), newSetCommand(d), newUnsetCommand(d), newListCommand(d))
	return cmd
}

func newGetCommand(d Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "get KEY", Short: "Print a configuration value", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runGet(d, args[0])
	}
	return cmd
}

func newSetCommand(d Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "set KEY VALUE", Short: "Set a configuration value", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(2)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSet(d, args[0], args[1])
	}
	return cmd
}

func newUnsetCommand(d Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "unset KEY", Short: "Remove a configuration value", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runUnset(d, args[0])
	}
	return cmd
}

func newListCommand(d Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List configuration values", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runList(d)
	}
	return cmd
}

func runGet(d Deps, key string) error {
	if err := checkKey(key); err != nil {
		return err
	}
	file, _, profile, err := loadProfile(d)
	if err != nil {
		return err
	}
	fmt.Fprintln(d.Streams.Out, configValue(file.Fields(profile), key))
	return nil
}

func runSet(d Deps, key, value string) error {
	if err := checkKey(key); err != nil {
		return err
	}
	file, store, profile, err := loadProfile(d)
	if err != nil {
		return err
	}
	file.Set(profile, key, value)
	return store.Save(file)
}

func runUnset(d Deps, key string) error {
	if err := checkKey(key); err != nil {
		return err
	}
	file, store, profile, err := loadProfile(d)
	if err != nil {
		return err
	}
	file.Unset(profile, key)
	return store.Save(file)
}

func runList(d Deps) error {
	file, _, profile, err := loadProfile(d)
	if err != nil {
		return err
	}
	fields := file.Fields(profile)
	fmt.Fprintf(d.Streams.Out, "%s=%s\n", credentials.KeyOrganizationID, fields.OrganizationID)
	fmt.Fprintf(d.Streams.Out, "%s=%s\n", credentials.KeyNamespaceID, fields.NamespaceID)
	fmt.Fprintf(d.Streams.Out, "%s=%s\n", credentials.KeyAPIDomain, fields.APIDomain)
	return nil
}

func loadProfile(d Deps) (*credentials.File, credentials.Store, string, error) {
	store, err := openStore(d)
	if err != nil {
		return nil, credentials.Store{}, "", err
	}
	file, err := store.Load()
	if err != nil {
		return nil, credentials.Store{}, "", err
	}
	return file, store, profileName(d), nil
}

func openStore(d Deps) (credentials.Store, error) {
	if d.Store == nil {
		return credentials.Store{}, ErrNoStore
	}
	return d.Store()
}

func profileName(d Deps) string {
	if d.Profile != nil {
		if name := d.Profile(); name != "" {
			return name
		}
	}
	return credentials.DefaultProfile
}

func checkKey(key string) error {
	switch key {
	case credentials.KeyOrganizationID, credentials.KeyNamespaceID, credentials.KeyAPIDomain:
		return nil
	default:
		return &cli.UsageError{Msg: fmt.Sprintf("unknown config key %q; must be organization_id, namespace_id, or api_domain", key)}
	}
}

func configValue(fields credentials.Profile, key string) string {
	switch key {
	case credentials.KeyOrganizationID:
		return fields.OrganizationID
	case credentials.KeyNamespaceID:
		return fields.NamespaceID
	case credentials.KeyAPIDomain:
		return fields.APIDomain
	}
	return ""
}
