package apiclient

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

func (c *Client) Stream(ctx context.Context, r Request, onEvent func(name string, data []byte) error) error {
	body, err := c.openStream(ctx, r)
	if err != nil {
		return err
	}
	defer body.Close()
	return parseSSE(ctx, body, onEvent)
}

func (c *Client) StreamLines(ctx context.Context, r Request, onLine func(line []byte) error) error {
	body, err := c.openStream(ctx, r)
	if err != nil {
		return err
	}
	defer body.Close()
	return parseLines(ctx, body, onLine)
}

func (c *Client) openStream(ctx context.Context, r Request) (io.ReadCloser, error) {
	req, err := c.newHTTPRequest(ctx, r)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("apiclient: stream %s %s: %w", r.Method, r.Path, err)
	}
	c.logStatus(r, req, resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		defer resp.Body.Close()
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, fmt.Errorf("apiclient: stream %s %s: %w", r.Method, r.Path, readErr)
		}
		return nil, normalizeError(resp.StatusCode, body, r.Product, r.Method, r.Path, resp.Header.Get("X-Request-Id"))
	}
	return resp.Body, nil
}

func parseLines(ctx context.Context, r io.Reader, onLine func(line []byte) error) error {
	reader := bufio.NewReader(r)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := reader.ReadBytes('\n')
		eof := errors.Is(err, io.EOF)
		if err != nil && !eof {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return fmt.Errorf("apiclient: stream lines: %w", err)
		}
		line = bytes.TrimRight(line, "\r\n")
		if len(bytes.TrimSpace(line)) > 0 {
			if callbackErr := onLine(line); callbackErr != nil {
				return callbackErr
			}
		}
		if eof {
			return nil
		}
	}
}

func parseSSE(ctx context.Context, r io.Reader, onEvent func(name string, data []byte) error) error {
	br := bufio.NewReader(r)
	var event string
	var data []string
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := br.ReadString('\n')
		eof := errors.Is(err, io.EOF)
		if err != nil && !eof {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return fmt.Errorf("apiclient: stream: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if dispatchErr := dispatchSSE(event, data, onEvent); dispatchErr != nil {
				return dispatchErr
			}
			event = ""
			data = nil
			if eof {
				return nil
			}
			continue
		}
		if !strings.HasPrefix(line, ":") {
			field, value, _ := strings.Cut(line, ":")
			if value != "" && value[0] == ' ' {
				value = value[1:]
			}
			switch field {
			case "event":
				event = value
			case "data":
				data = append(data, value)
			}
		}
		if eof {
			return dispatchSSE(event, data, onEvent)
		}
	}
}

func dispatchSSE(event string, data []string, onEvent func(name string, data []byte) error) error {
	if len(data) == 0 {
		return nil
	}
	if event == "" {
		event = "message"
	}
	return onEvent(event, []byte(strings.Join(data, "\n")))
}
