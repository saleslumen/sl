package apiclient

import (
	"context"
	"fmt"
)

const maxListPages = 1000

func ListAll[T any](ctx context.Context, limit int, fetch func(ctx context.Context, pageToken string) (items []T, next string, err error)) ([]T, error) {
	var out []T
	token := ""
	for page := 0; page < maxListPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items, next, err := fetch(ctx, token)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if limit > 0 && len(out) >= limit {
			return out[:limit], nil
		}
		if next == "" || next == token {
			return out, nil
		}
		token = next
	}
	return nil, fmt.Errorf("apiclient: pagination exceeded %d pages", maxListPages)
}
