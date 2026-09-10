package campaigns

import (
	"bytes"
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
	metricsFixture  = `{"campaign":"campaigns/c1","time_range_start":"2026-08-01T00:00:00Z","time_range_end":"2026-09-01T00:00:00Z","reporting_timezone":"America/New_York","enrolled_recipients":{"count":12,"rate":null,"numerator":"people","denominator":"campaign","unavailable":false},"scheduled_messages":{"count":10,"rate":0.5,"numerator":"scheduled","denominator":"enrolled","unavailable":false},"accepted_messages":{"count":9,"rate":0.9,"numerator":"accepted","denominator":"scheduled","unavailable":false},"attempted_messages":{"count":8,"rate":0.8,"numerator":"attempted","denominator":"accepted","unavailable":false},"sent_messages":{"count":7,"rate":0.7,"numerator":"sent","denominator":"attempted","unavailable":false},"delivered_messages":{"count":6,"rate":0.6,"numerator":"delivered","denominator":"sent","unavailable":false},"bounced_messages":{"count":1,"rate":0.1,"numerator":"bounced","denominator":"sent","unavailable":false},"unique_opened":{"count":null,"rate":null,"numerator":"opened","denominator":"delivered","unavailable":true},"unique_clicked":{"count":2,"rate":0.25,"numerator":"clicked","denominator":"opened","unavailable":false},"unique_replied":{"count":1,"rate":0.125,"numerator":"replied","denominator":"delivered","unavailable":false},"cancelled_messages":{"count":0,"rate":0,"numerator":"cancelled","denominator":"scheduled","unavailable":false},"failed_messages":{"count":1,"rate":0.1,"numerator":"failed","denominator":"attempted","unavailable":false}}`
	metricsTable    = "enrolled_recipients\t12\t\tpeople\tcampaign\tfalse\n" +
		"scheduled_messages\t10\t0.5\tscheduled\tenrolled\tfalse\n" +
		"accepted_messages\t9\t0.9\taccepted\tscheduled\tfalse\n" +
		"attempted_messages\t8\t0.8\tattempted\taccepted\tfalse\n" +
		"sent_messages\t7\t0.7\tsent\tattempted\tfalse\n" +
		"delivered_messages\t6\t0.6\tdelivered\tsent\tfalse\n" +
		"bounced_messages\t1\t0.1\tbounced\tsent\tfalse\n" +
		"unique_opened\t\t\topened\tdelivered\ttrue\n" +
		"unique_clicked\t2\t0.25\tclicked\topened\tfalse\n" +
		"unique_replied\t1\t0.125\treplied\tdelivered\tfalse\n" +
		"cancelled_messages\t0\t0\tcancelled\tscheduled\tfalse\n" +
		"failed_messages\t1\t0.1\tfailed\tattempted\tfalse\n"
	metricsRequiredQuery = "reporting_timezone=America%2FNew_York&time_range_end=2026-09-01T00%3A00%3A00Z&time_range_start=2026-08-01T00%3A00%3A00Z"
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

func TestMetricsGetUsageAnd502(t *testing.T) {
	f := deliveriesFactory(t, nil, io.Discard, false)
	f.Client = func() (*apiclient.Client, error) {
		t.Fatal("client")
		return nil, nil
	}
	assertUsageErr(t, executeMetrics(t, f, "get"), metricsRequired)
	assertUsageErr(t, executeMetrics(t, f, "get", "--campaign", "c1"), "--time-range-start")
	assertUsageErr(t, executeMetrics(t, f, "get", "--campaign", "c1", "--time-range-start", metricsStart), "--time-range-end")
	assertUsageErr(t, executeMetrics(t, f, "get", "--campaign", "c1", "--time-range-start", metricsStart, "--time-range-end", metricsEnd), "--reporting-timezone")
	srv, _ := captureCampaigns(t, writeStatus(http.StatusBadGateway, analytics502))
	ok := deliveriesFactory(t, nil, io.Discard, false)
	ok.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	assertUnavailable(t, executeMetrics(t, ok, metricsArgs()...), http.MethodGet, "/v1/campaigns/c1/metrics")
}

func TestMetricsGetDecodeError(t *testing.T) {
	srv, _ := captureCampaigns(t, writeOK(`[]`))
	f := deliveriesFactory(t, nil, io.Discard, false)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeMetrics(t, f, metricsArgs()...); err == nil || !strings.Contains(err.Error(), "decode metrics") {
		t.Fatalf("decode: %v", err)
	}
}
