package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrMixedCredentials = errors.New("API key and access token must not be combined")

const (
	defaultAPIDomain = "saleslumenapis.com"
	defaultUserAgent = "sl"
	requestTimeout   = 30 * time.Second
)

type Options struct {
	APIKey      string
	AccessToken string
	NamespaceID string
	APIDomain   string
	UserAgent   string
	HTTPClient  *http.Client
	Logger      func(string)
}

type Request struct {
	Product string
	Method  string
	Path    string
	Query   url.Values
	Body    any
}

type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

type Client struct {
	apiKey      string
	accessToken string
	namespaceID string
	apiDomain   string
	userAgent   string
	http        *http.Client
	log         func(string)
}

func New(o Options) *Client {
	httpClient := o.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	apiDomain := strings.TrimSpace(o.APIDomain)
	if apiDomain == "" {
		apiDomain = defaultAPIDomain
	}
	userAgent := o.UserAgent
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	return &Client{apiKey: o.APIKey, accessToken: o.AccessToken, namespaceID: o.NamespaceID, apiDomain: apiDomain, userAgent: userAgent, http: httpClient, log: o.Logger}
}

func (c *Client) Do(ctx context.Context, r Request) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := c.newHTTPRequest(ctx, r)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("apiclient: do %s %s: %w", r.Method, r.Path, err)
	}
	defer resp.Body.Close()
	c.logStatus(r, req, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("apiclient: read %s %s: %w", r.Method, r.Path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, normalizeError(resp.StatusCode, body, r.Product, r.Method, r.Path, resp.Header.Get("X-Request-Id"))
	}
	return &Response{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: body}, nil
}

func (c *Client) newHTTPRequest(ctx context.Context, r Request) (*http.Request, error) {
	if r.Product == "" {
		return nil, fmt.Errorf("apiclient: product is required")
	}
	if r.Method == "" {
		return nil, fmt.Errorf("apiclient: method is required")
	}
	if r.Path == "" || !strings.HasPrefix(r.Path, "/") {
		return nil, fmt.Errorf("apiclient: path must begin with /")
	}
	var bodyReader io.Reader
	hasBody := r.Body != nil
	if hasBody {
		encoded, err := encodeBody(r.Body)
		if err != nil {
			return nil, fmt.Errorf("apiclient: encode body: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
	}
	u := url.URL{Scheme: "https", Host: r.Product + "." + c.apiDomain, Path: r.Path, RawQuery: r.Query.Encode()}
	req, err := http.NewRequestWithContext(ctx, r.Method, u.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("apiclient: new request: %w", err)
	}
	if err := c.applyAuth(req); err != nil {
		return nil, err
	}
	if c.namespaceID != "" {
		req.Header.Set("sl-namespace-id", c.namespaceID)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) applyAuth(req *http.Request) error {
	if c.apiKey != "" && c.accessToken != "" {
		return fmt.Errorf("apiclient: %w", ErrMixedCredentials)
	}
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
		return nil
	}
	if c.apiKey != "" {
		req.Header.Set("sl-api-key", c.apiKey)
	}
	return nil
}

// A []byte body is already-encoded JSON and is sent verbatim; json.Marshal would base64-encode it.
func encodeBody(body any) ([]byte, error) {
	if raw, ok := body.([]byte); ok {
		return raw, nil
	}
	return json.Marshal(body)
}

func (c *Client) logStatus(r Request, req *http.Request, status int) {
	if c.log == nil {
		return
	}
	c.log(fmt.Sprintf("%s %s %s → %d", r.Method, req.URL.Host, r.Path, status))
}
