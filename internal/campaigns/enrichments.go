package campaigns

import (
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/spf13/cobra"
	"net/http"
	"net/url"
)

func newEnrichmentsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "enrichments", Short: "Run saved campaign enrichments", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }}
	var campaign, source, requestID string
	run := &cobra.Command{Use: "run --campaign ID --input FILE", Short: "Run saved enrichments", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	peopleBindCampaign(run, &campaign)
	peopleBindInput(run, &source)
	peopleBindRequestID(run, &requestID)
	run.RunE = func(cmd *cobra.Command, _ []string) error {
		campaignID, err := requireCampaign(campaign)
		if err != nil {
			return err
		}
		body, err := readRequiredObject(f, source)
		if err != nil {
			return err
		}
		if err = ensureRequestID(f, body, requestID, cmd.Flags().Changed("request-id")); err != nil {
			return err
		}
		response, err := do(cmd.Context(), f, http.MethodPost, "/v1/campaigns/"+campaignID+"/enrichments:run", nil, body)
		if err != nil {
			return err
		}
		return f.Printer().Object(response.Body)
	}
	cmd.AddCommand(run)
	for _, action := range []string{"list", "get", "create", "update", "delete"} {
		cmd.AddCommand(newEnrichmentResourceCommand(f, action))
	}
	return cmd
}

func newEnrichmentResourceCommand(f *cli.Factory, action string) *cobra.Command {
	var campaign, source, requestID, etag, pageToken string
	var pageSize int
	args := cobra.NoArgs
	if action == "get" || action == "update" || action == "delete" {
		args = exactArg("enrichment id")
	}
	cmd := &cobra.Command{Use: action, Short: action + " saved enrichments", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: args}
	peopleBindCampaign(cmd, &campaign)
	if action == "create" || action == "update" {
		peopleBindInput(cmd, &source)
	}
	if action == "create" || action == "update" || action == "delete" {
		peopleBindRequestID(cmd, &requestID)
	}
	if action == "list" {
		cmd.Flags().IntVar(&pageSize, "page-size", 50, "Maximum results per page (at most 200)")
		cmd.Flags().StringVar(&pageToken, "page-token", "", "Page token")
	}
	if action == "delete" {
		cmd.Flags().StringVar(&etag, "etag", "", "Enrichment etag")
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		campaignID, err := requireCampaign(campaign)
		if err != nil {
			return err
		}
		path := "/v1/campaigns/" + campaignID + "/enrichments"
		if len(args) > 0 {
			path += "/" + url.PathEscape(resourceID(args[0]))
		}
		method := http.MethodGet
		var body any
		var query url.Values
		switch action {
		case "list":
			if pageSize < 1 || pageSize > 200 {
				return &cli.UsageError{Msg: "--page-size must be between 1 and 200"}
			}
			query = listQuery(pageSize, pageToken)
		case "create", "update":
			obj, err := readRequiredObject(f, source)
			if err != nil {
				return err
			}
			if err = ensureRequestID(f, obj, requestID, cmd.Flags().Changed("request-id")); err != nil {
				return err
			}
			body = obj
			if action == "create" {
				method = http.MethodPost
			} else {
				method = http.MethodPatch
			}
		case "delete":
			if etag == "" {
				return &cli.UsageError{Msg: "--etag is required"}
			}
			if err = f.Confirm("Delete enrichment?"); err != nil {
				return err
			}
			obj := input.Object{}
			if err = ensureRequestID(f, obj, requestID, cmd.Flags().Changed("request-id")); err != nil {
				return err
			}
			query = url.Values{"etag": {etag}, "request_id": {obj.String("request_id")}}
			method = http.MethodDelete
		}
		response, err := do(cmd.Context(), f, method, path, query, body)
		if err != nil {
			return err
		}
		if action == "delete" {
			return nil
		}
		return f.Printer().Object(response.Body)
	}
	return cmd
}
