package campaigns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/spf13/cobra"
)

func newRulesCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "rules", Short: "Manage campaign rules", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}}
	cmd.AddCommand(newRulesListCommand(f), newRulesGetCommand(f), newRulesCreateCommand(f), newRulesUpdateCommand(f), newRulesDeleteCommand(f))
	return cmd
}

func newRulesListCommand(f *cli.Factory) *cobra.Command {
	var campaign string
	limit := 50
	cmd := &cobra.Command{Use: "list --campaign ID", Short: "List campaign rules", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&campaign, "campaign", "", "Parent campaign ID")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rules to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runRulesList(cmd.Context(), f, campaign, limit)
	}
	return cmd
}

func newRulesGetCommand(f *cli.Factory) *cobra.Command {
	var campaign string
	cmd := &cobra.Command{Use: "get ID --campaign ID", Short: "Get a campaign rule", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("rule id")}
	cmd.Flags().StringVar(&campaign, "campaign", "", "Parent campaign ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		path, err := campaignRulePath(campaign, args[0])
		if err != nil {
			return err
		}
		resp, err := do(cmd.Context(), f, http.MethodGet, path, nil, nil)
		if err != nil {
			return err
		}
		return printCampaignJSON(f, resp.Body)
	}
	return cmd
}

func newRulesCreateCommand(f *cli.Factory) *cobra.Command {
	var campaign, source, requestID string
	cmd := &cobra.Command{Use: "create --campaign ID --input FILE", Short: "Create a campaign rule", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&campaign, "campaign", "", "Parent campaign ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runRulesCreate(cmd.Context(), f, campaign, source, requestID, cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newRulesUpdateCommand(f *cli.Factory) *cobra.Command {
	var campaign, source, requestID, etag string
	cmd := &cobra.Command{Use: "update ID --campaign ID --input FILE", Short: "Update a campaign rule", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("rule id")}
	cmd.Flags().StringVar(&campaign, "campaign", "", "Parent campaign ID")
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.Flags().StringVar(&etag, "etag", "", "Rule etag; fetched from the rule when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runRulesUpdate(cmd.Context(), f, campaign, args[0], source, requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"))
	}
	return cmd
}

func newRulesDeleteCommand(f *cli.Factory) *cobra.Command {
	var campaign, requestID, etag string
	cmd := &cobra.Command{Use: "delete ID --campaign ID", Short: "Delete a campaign rule", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("rule id")}
	cmd.Flags().StringVar(&campaign, "campaign", "", "Parent campaign ID")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.Flags().StringVar(&etag, "etag", "", "Rule etag; fetched from the rule when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runRulesDelete(cmd.Context(), f, campaign, args[0], requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"))
	}
	return cmd
}

func runRulesList(ctx context.Context, f *cli.Factory, campaign string, limit int) error {
	id, err := requireCampaign(campaign)
	if err != nil {
		return err
	}
	if limit < 1 {
		return &cli.UsageError{Msg: "--limit must be at least 1"}
	}
	fetched := 0
	items, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]json.RawMessage, string, error) {
		resp, err := do(ctx, f, http.MethodGet, "/v1/campaigns/"+id+"/rules", listQuery(limit-fetched, pageToken), nil)
		if err != nil {
			return nil, "", err
		}
		page, next, err := apiclient.DecodePage[json.RawMessage](resp.Body, "rules", "next_page_token")
		if err != nil {
			return nil, "", err
		}
		fetched += len(page)
		return page, next, nil
	})
	if err != nil {
		return err
	}
	if items == nil {
		items = []json.RawMessage{}
	}
	raw, err := json.Marshal(struct {
		Rules []json.RawMessage `json:"rules"`
	}{Rules: items})
	if err != nil {
		return fmt.Errorf("campaigns: encode rules: %w", err)
	}
	return printCampaignJSON(f, raw)
}

func runRulesCreate(ctx context.Context, f *cli.Factory, campaign, source, requestID string, requestIDSet bool) error {
	id, err := requireCampaign(campaign)
	if err != nil {
		return err
	}
	body, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := ensureRequestID(f, body, requestID, requestIDSet); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, "/v1/campaigns/"+id+"/rules", nil, body)
	if err != nil {
		return err
	}
	return printCampaignJSON(f, resp.Body)
}

func runRulesUpdate(ctx context.Context, f *cli.Factory, campaign, rule, source, requestID, etag string, requestIDSet, etagSet bool) error {
	path, err := campaignRulePath(campaign, rule)
	if err != nil {
		return err
	}
	body, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := ensureRequestID(f, body, requestID, requestIDSet); err != nil {
		return err
	}
	if err := ensureEtag(ctx, f, body, etag, etagSet, path); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPatch, path, nil, body)
	if err != nil {
		return err
	}
	return printCampaignJSON(f, resp.Body)
}

func runRulesDelete(ctx context.Context, f *cli.Factory, campaign, rule, requestID, etag string, requestIDSet, etagSet bool) error {
	path, err := campaignRulePath(campaign, rule)
	if err != nil {
		return err
	}
	if err := f.Confirm("Delete campaign rule " + resourceID(rule) + "?"); err != nil {
		return err
	}
	controls := input.Object{}
	if err := ensureRequestID(f, controls, requestID, requestIDSet); err != nil {
		return err
	}
	if err := ensureEtag(ctx, f, controls, etag, etagSet, path); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("request_id", controls.String("request_id"))
	query.Set("etag", controls.String("etag"))
	if _, err := do(ctx, f, http.MethodDelete, path, query, nil); err != nil {
		return err
	}
	return nil
}

func campaignRulePath(campaign, rule string) (string, error) {
	campaignID, err := requireCampaign(campaign)
	if err != nil {
		return "", err
	}
	ruleID, err := requireArg([]string{rule}, "rule id")
	if err != nil {
		return "", err
	}
	return "/v1/campaigns/" + campaignID + "/rules/" + resourceID(ruleID), nil
}
