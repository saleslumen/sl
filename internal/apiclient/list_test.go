package apiclient

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestListAllStopsAtLimit(t *testing.T) {
	calls := 0
	items, err := ListAll(context.Background(), 3, func(ctx context.Context, pageToken string) ([]string, string, error) {
		calls++
		switch pageToken {
		case "":
			return []string{"a", "b"}, "p2", nil
		case "p2":
			return []string{"c", "d"}, "p3", nil
		default:
			return []string{"e"}, "", nil
		}
	})
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls: %d", calls)
	}
	if len(items) != 3 || items[0] != "a" || items[1] != "b" || items[2] != "c" {
		t.Fatalf("items: %#v", items)
	}
}

func TestListAllExhaustsPages(t *testing.T) {
	items, err := ListAll(context.Background(), 0, func(ctx context.Context, pageToken string) ([]string, string, error) {
		if pageToken == "" {
			return []string{"a"}, "next", nil
		}
		return []string{"b"}, "", nil
	})
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(items) != 2 || items[0] != "a" || items[1] != "b" {
		t.Fatalf("items: %#v", items)
	}
}

func TestListAllStopsOnRepeatedToken(t *testing.T) {
	calls := 0
	items, err := ListAll(context.Background(), 0, func(ctx context.Context, pageToken string) ([]string, string, error) {
		calls++
		return []string{"a"}, "same", nil
	})
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls: %d", calls)
	}
	if len(items) != 2 {
		t.Fatalf("items: %#v", items)
	}
}

func TestListAllPropagatesFetchError(t *testing.T) {
	want := errors.New("boom")
	_, err := ListAll(context.Background(), 10, func(ctx context.Context, pageToken string) ([]string, string, error) {
		return nil, "", want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err: %v", err)
	}
}

func TestListAllHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ListAll(ctx, 10, func(ctx context.Context, pageToken string) ([]string, string, error) {
		t.Fatal("fetch called")
		return nil, "", nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err: %v", err)
	}
}

func TestListAllEnforcesPageCap(t *testing.T) {
	calls := 0
	_, err := ListAll(context.Background(), 0, func(ctx context.Context, pageToken string) ([]string, string, error) {
		calls++
		return []string{pageToken}, strconv.Itoa(calls), nil
	})
	if err == nil || !strings.Contains(err.Error(), "pagination exceeded 1000 pages") {
		t.Fatalf("err=%v", err)
	}
	if calls != maxListPages {
		t.Fatalf("calls=%d", calls)
	}
}
