package campaigns

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

const (
	metricsRequired = "--campaign"
	metricsStart    = "2026-08-01T00:00:00Z"
	metricsEnd      = "2026-09-01T00:00:00Z"
	metricsTZ       = "America/New_York"
	metricsFixture  = `{"campaign":"campaigns/c1","time_range_start":"2026-08-01T00:00:00Z","time_range_end":"2026-09-01T00:00:00Z","reporting_timezone":"America/New_York","sent_messages":{"count":7,"numerator":"sent_messages","denominator":"none","unavailable":false},"bounced_messages":{"count":1,"rate":0.1,"numerator":"bounced_messages","denominator":"sent_messages","unavailable":false},"unique_opened":{"count":0,"numerator":"unique_opened","denominator":"sent_messages_tracking_enabled","unavailable":true},"unique_clicked":{"count":2,"rate":0.25,"numerator":"unique_clicked","denominator":"sent_messages_tracking_enabled","unavailable":false},"unique_replied":{"count":1,"rate":0.125,"numerator":"unique_replied","denominator":"sent_messages","unavailable":false},"people":{"snapshot_time":"2026-08-12T00:00:00Z","active":4,"paused":1,"not_sent":2,"finished":3},"steps":[{"sequence":"campaigns/c1/sequences/s1","step":"campaigns/c1/sequences/s1/steps/st1","scheduled_messages":{"count":5,"numerator":"scheduled_messages","denominator":"none","unavailable":false},"sent_messages":{"count":4,"numerator":"sent_messages","denominator":"none","unavailable":false},"bounced_messages":{"count":0,"numerator":"bounced_messages","denominator":"sent_messages","unavailable":false},"unique_replied":{"count":1,"numerator":"unique_replied","denominator":"sent_messages","unavailable":false},"interested_people":{"count":1,"numerator":"interested_people","denominator":"none","unavailable":false},"variants":[{"variant":"campaigns/c1/sequences/s1/steps/st1/variants/v1","scheduled_messages":{"count":5,"numerator":"scheduled_messages","denominator":"none","unavailable":false},"sent_messages":{"count":4,"numerator":"sent_messages","denominator":"none","unavailable":false},"bounced_messages":{"count":0,"numerator":"bounced_messages","denominator":"sent_messages","unavailable":false},"unique_replied":{"count":1,"numerator":"unique_replied","denominator":"sent_messages","unavailable":false}}]}]}`
	metricsTable    = "sent_messages\t7\t\tsent_messages\tnone\tfalse\n" +
		"bounced_messages\t1\t0.1\tbounced_messages\tsent_messages\tfalse\n" +
		"unique_opened\t0\t\tunique_opened\tsent_messages_tracking_enabled\ttrue\n" +
		"unique_clicked\t2\t0.25\tunique_clicked\tsent_messages_tracking_enabled\tfalse\n" +
		"unique_replied\t1\t0.125\tunique_replied\tsent_messages\tfalse\n"
	metricsRequiredQuery = "reporting_timezone=America%2FNew_York&time_range_end=2026-09-01T00%3A00%3A00Z&time_range_start=2026-08-01T00%3A00%3A00Z"
	metricsUnavailable   = `{"code":"UNAVAILABLE","message":"analytics unavailable","details":[{"reason":"ANALYTICS_UNAVAILABLE","domain":"campaigns"}]}`
)

func executeMetrics(t *testing.T, f *cli.Factory, args ...string) error {
	t.Helper()
	cmd := newMetricsCommand(f)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
}

func metricsArgs(extra ...string) []string {
	args := []string{"get", "--campaign", "c1", "--time-range-start", metricsStart, "--time-range-end", metricsEnd, "--reporting-timezone", metricsTZ}
	return append(args, extra...)
}

func TestMetricsCommandWiring(t *testing.T) {
	var stdout bytes.Buffer
	called := false
	f := deliveriesFactory(t, nil, &stdout, false)
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, cli.ErrNoCredentials
	}
	cmd := newMetricsCommand(f)
	if cmd.Use != "metrics" || cmd.Short != "Read campaign metrics" || strings.Contains(cmd.Short, "\n") {
		t.Fatalf("use=%q short=%q", cmd.Use, cmd.Short)
	}
	var get *cobra.Command
	for _, child := range cmd.Commands() {
		if child.Name() == "get" {
			get = child
		}
	}
	if get == nil || get.Use != "get --campaign ID --time-range-start TIME --time-range-end TIME --reporting-timezone TZ" || get.Short != "Get campaign metrics" || strings.Contains(get.Short, "\n") {
		t.Fatalf("get=%v use=%q", get, get.Use)
	}
	for _, flag := range []string{"campaign", "time-range-start", "time-range-end", "reporting-timezone", "step-id", "variant-id", "sender-account-id"} {
		if get.Flags().Lookup(flag) == nil {
			t.Fatalf("get missing --%s", flag)
		}
	}
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	if called {
		t.Fatal("help resolved a lazy dependency")
	}
	if !strings.Contains(stdout.String(), "get") {
		t.Fatalf("help=%q", stdout.String())
	}
}

func TestMetricsGetRequiredQueryHeadersAndTable(t *testing.T) {
	srv, recs := captureCampaigns(t, writeOK(metricsFixture))
	var stdout bytes.Buffer
	f := deliveriesFactory(t, nil, &stdout, false)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeMetrics(t, f, metricsArgs()...); err != nil {
		t.Fatalf("get: %v", err)
	}
	assertCampaignRequest(t, (*recs)[0], http.MethodGet, "/v1/campaigns/c1/metrics", metricsRequiredQuery, "", "", false)
	if stdout.String() != metricsTable {
		t.Fatalf("stdout=%q want=%q", stdout.String(), metricsTable)
	}
	var decoded campaignMetrics
	if err := json.Unmarshal([]byte(metricsFixture), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.People.Active != 4 || decoded.People.NotSent != 2 || len(decoded.Steps) != 1 || decoded.Steps[0].Step != "campaigns/c1/sequences/s1/steps/st1" || decoded.Steps[0].InterestedPeople == nil {
		t.Fatalf("decoded people/steps %+v", decoded)
	}
}

func TestMetricsGetOptionalFiltersNamespaceAndJSON(t *testing.T) {
	srv, recs := captureCampaigns(t, writeOK(metricsFixture))
	var stdout bytes.Buffer
	f := deliveriesFactory(t, nil, &stdout, true)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, "ns-7"), nil }
	if err := executeMetrics(t, f, metricsArgs("--step-id", "s1", "--variant-id", "v1", "--sender-account-id", "sa1")...); err != nil {
		t.Fatalf("get: %v", err)
	}
	wantQuery := "reporting_timezone=America%2FNew_York&sender_account_id=sa1&step_id=s1&time_range_end=2026-09-01T00%3A00%3A00Z&time_range_start=2026-08-01T00%3A00%3A00Z&variant_id=v1"
	assertCampaignRequest(t, (*recs)[0], http.MethodGet, "/v1/campaigns/c1/metrics", wantQuery, "", "ns-7", false)
	if stdout.String() != metricsFixture {
		t.Fatalf("json=%q", stdout.String())
	}
}

func TestMetricsGetOmitsBlankOptionalFilters(t *testing.T) {
	srv, recs := captureCampaigns(t, writeOK(`{"campaign":"campaigns/c1","time_range_start":"2026-08-01T00:00:00Z","time_range_end":"2026-09-01T00:00:00Z","reporting_timezone":"UTC"}`))
	var stdout bytes.Buffer
	f := deliveriesFactory(t, nil, &stdout, false)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeMetrics(t, f, "get", "--campaign", "c1", "--time-range-start", metricsStart, "--time-range-end", metricsEnd, "--reporting-timezone", "UTC"); err != nil {
		t.Fatalf("get: %v", err)
	}
	assertCampaignRequest(t, (*recs)[0], http.MethodGet, "/v1/campaigns/c1/metrics", "reporting_timezone=UTC&time_range_end=2026-09-01T00%3A00%3A00Z&time_range_start=2026-08-01T00%3A00%3A00Z", "", "", false)
	if stdout.String() != "" {
		t.Fatalf("empty metrics stdout=%q", stdout.String())
	}
}

func TestMetricsGetUsageAndUnavailable(t *testing.T) {
	f := deliveriesFactory(t, nil, io.Discard, false)
	f.Client = func() (*apiclient.Client, error) {
		t.Fatal("client")
		return nil, nil
	}
	assertUsageErr(t, executeMetrics(t, f, "get"), metricsRequired)
	assertUsageErr(t, executeMetrics(t, f, "get", "--campaign", "c1"), "--time-range-start")
	assertUsageErr(t, executeMetrics(t, f, "get", "--campaign", "c1", "--time-range-start", metricsStart), "--time-range-end")
	assertUsageErr(t, executeMetrics(t, f, "get", "--campaign", "c1", "--time-range-start", metricsStart, "--time-range-end", metricsEnd), "--reporting-timezone")
	srv, _ := captureCampaigns(t, writeStatus(http.StatusServiceUnavailable, metricsUnavailable))
	ok := deliveriesFactory(t, nil, io.Discard, false)
	ok.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	assertMetricsUnavailable(t, executeMetrics(t, ok, metricsArgs()...), http.MethodGet, "/v1/campaigns/c1/metrics")
}

func assertMetricsUnavailable(t *testing.T, err error, method, path string) {
	t.Helper()
	var api *apiclient.Error
	if !errors.As(err, &api) || api.Status != http.StatusServiceUnavailable || api.Code != "UNAVAILABLE" || api.Message != "analytics unavailable" || api.Product != "campaigns" || api.Method != method || api.Path != path {
		t.Fatalf("503: %#v", err)
	}
	var details []struct {
		Reason string `json:"reason"`
		Domain string `json:"domain"`
	}
	if json.Unmarshal(api.Details, &details) != nil || len(details) != 1 || details[0].Reason != "ANALYTICS_UNAVAILABLE" || details[0].Domain != "campaigns" {
		t.Fatalf("details=%s", api.Details)
	}
}

func TestMetricsGetDecodeError(t *testing.T) {
	srv, _ := captureCampaigns(t, writeOK(`[]`))
	f := deliveriesFactory(t, nil, io.Discard, false)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeMetrics(t, f, metricsArgs()...); err == nil || !strings.Contains(err.Error(), "decode metrics") {
		t.Fatalf("decode: %v", err)
	}
}
