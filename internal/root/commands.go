package root

import (
	"context"
	"net/url"
	"os"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/auth"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/config"
	"github.com/saleslumen/sl/internal/credentials"
	"github.com/spf13/cobra"
)

func newAuthCommand(f *cli.Factory, command *cobra.Command) *cobra.Command {
	streams := auth.Streams{In: f.IO.In, Out: f.IO.Out, Err: f.IO.Err, IsStdinTTY: f.IO.IsStdinTTY}
	return auth.NewCommand(auth.Deps{
		Streams: streams,
		Store:   resolveStore,
		Profile: func() string {
			return firstNonEmpty(flagValue(command, "profile"), os.Getenv(credentials.EnvProfile), credentials.DefaultProfile)
		},
		Organization: func() string {
			return firstNonEmpty(flagValue(command, "organization"), os.Getenv(credentials.EnvOrganizationID))
		},
		Resolve: func() (credentials.Resolved, error) {
			return resolveFromRoot(command)
		},
		Probe: func(ctx context.Context, product, method, path string, query url.Values) error {
			return probeProduct(ctx, f, product, method, path, query)
		},
		ReadSecret: func() (string, error) {
			return auth.ReadTerminalSecret(streams)
		},
	})
}

func newConfigCommand(f *cli.Factory, command *cobra.Command) *cobra.Command {
	return config.NewCommand(config.Deps{
		Streams: config.Streams{Out: f.IO.Out},
		Store:   resolveStore,
		Profile: func() string {
			return firstNonEmpty(flagValue(command, "profile"), os.Getenv(credentials.EnvProfile), credentials.DefaultProfile)
		},
	})
}

func probeProduct(ctx context.Context, f *cli.Factory, product, method, path string, query url.Values) error {
	if f.Client == nil {
		return cli.ErrNoCredentials
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	_, err = client.Do(ctx, apiclient.Request{Product: product, Method: method, Path: path, Query: query})
	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
