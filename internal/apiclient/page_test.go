package apiclient

import (
	"strings"
	"testing"
)

type pageItem struct {
	Name string `json:"name"`
}

func TestDecodePage(t *testing.T) {
	items, next, err := DecodePage[pageItem]([]byte(`{"campaigns":[{"name":"one"},{"name":"two"}],"next_page_token":"next"}`), "campaigns", "next_page_token")
	if err != nil {
		t.Fatalf("DecodePage: %v", err)
	}
	if len(items) != 2 || items[0].Name != "one" || items[1].Name != "two" || next != "next" {
		t.Fatalf("items=%v next=%q", items, next)
	}
}

func TestDecodePageAbsentAndNullAreEmpty(t *testing.T) {
	for _, body := range []string{`{}`, `{"items":null,"nextPageToken":null}`} {
		items, next, err := DecodePage[pageItem]([]byte(body), "items", "nextPageToken")
		if err != nil {
			t.Fatalf("DecodePage: %v", err)
		}
		if len(items) != 0 || next != "" {
			t.Fatalf("items=%v next=%q", items, next)
		}
	}
}

func TestDecodePageRejectsInvalidFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "body", body: `not-json`},
		{name: "non-object", body: `null`},
		{name: "items", body: `{"items":{}}`},
		{name: "token", body: `{"items":[],"nextPageToken":42}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := DecodePage[pageItem]([]byte(tc.body), "items", "nextPageToken")
			if err == nil || !strings.Contains(err.Error(), "apiclient:") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
