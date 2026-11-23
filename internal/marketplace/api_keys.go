package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newAPIKeyOperationsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "api-key-operations", Short: "Complete an API-key operation", SilenceUsage: true, SilenceErrors: true}
	complete := &cobra.Command{Use: "complete OPERATION_ID CONNECTION", Short: "Complete an API-key operation", Args: cobra.ExactArgs(2), SilenceUsage: true, SilenceErrors: true}
	complete.RunE = func(cmd *cobra.Command, args []string) error {
		return runAPIKeyComplete(cmd.Context(), f, args[0], args[1])
	}
	cmd.AddCommand(complete)
	return cmd
}

func newConnectionsSetAPIKeyCommand(f *cli.Factory) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "set-api-key CONNECTION --api-key-file FILE", Short: "Set a connection API key", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().StringVar(&source, "api-key-file", "", "API key file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runConnectionsSetAPIKey(cmd.Context(), f, args[0], source)
	}
	return cmd
}

func runConnectionsSetAPIKey(ctx context.Context, f *cli.Factory, name, source string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	installationID, connectionID, err := parseConnectionName(name)
	if err != nil {
		return err
	}
	secret, err := readAPIKey(f.IO.In, source)
	if err != nil {
		return err
	}
	operationID, err := newOperationID()
	if err != nil {
		return err
	}
	client, err := f.Client()
	if err != nil {
		return err
	}
	initiateBody, err := json.Marshal(map[string]string{"operationId": operationID})
	if err != nil {
		return err
	}
	if _, err := do(ctx, client, marketplaceProduct, http.MethodPost, connectionResourcePath(installationID, connectionID)+":initiateApiKey", nil, initiateBody); err != nil {
		return err
	}
	bindBody, err := json.Marshal(map[string]any{"operationId": operationID, "credential": map[string]string{"apiKey": secret}})
	secret = ""
	if err != nil {
		return err
	}
	_, _ = do(ctx, client, scriptProduct, http.MethodPost, scriptBindRoute.Path, nil, bindBody)
	bindBody = nil
	resource := connectionResourceName(installationID, connectionID)
	completeBody, err := json.Marshal(map[string]string{"connection": resource, "operationId": operationID})
	if err != nil {
		return err
	}
	raw, completeErr := do(ctx, client, marketplaceProduct, http.MethodPost, "/v1/installationConnectionApiKeyOperations:complete", nil, completeBody)
	if completeErr != nil {
		return fmt.Errorf("marketplace: complete api key operation %s failed; run 'sl marketplace api-key-operations complete %s %s': %w", operationID, operationID, resource, completeErr)
	}
	row, err := decodeObject[apiKeyOutcome](raw.Body)
	if err != nil {
		return err
	}
	return printRows(f, raw.Body, []apiKeyOutcome{row}, outcomeColumns())
}

func runAPIKeyComplete(ctx context.Context, f *cli.Factory, operationID, name string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	operationID, err := requireID(operationID, "operation ID")
	if err != nil {
		return err
	}
	installationID, connectionID, err := parseConnectionName(name)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"connection": connectionResourceName(installationID, connectionID), "operationId": operationID})
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodPost, "/v1/installationConnectionApiKeyOperations:complete", nil, body)
	if err != nil {
		return err
	}
	row, err := decodeObject[apiKeyOutcome](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []apiKeyOutcome{row}, outcomeColumns())
}
