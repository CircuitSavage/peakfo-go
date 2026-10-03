// Package peak is a Go client for the Peak CAPTCHA-solving API
// (Cloudflare Turnstile + Cloudflare 5-second challenge).
//
// Verified against https://api.peak.fo (2026-10-03): request/response shapes,
// required fields, and error formats were checked live, not assumed from docs.
package peak

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the Peak API base.
const DefaultBaseURL = "https://api.peak.fo"

// Sentinel errors for the common HTTP-level failures.
var (
	ErrUnauthorized        = errors.New("peak: unauthorized (check API key)")
	ErrInsufficientBalance = errors.New("peak: insufficient balance")
)

// Client talks to the Peak API.
type Client struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
}

// New returns a client with a 60s timeout. Solves usually return in
// ~1-1.5s, but a managed challenge can take longer, so the default is
// generous.
func New(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

type apiResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
	Cost    float64         `json:"cost"`
}

// APIError wraps a success:false response body that isn't one of the
// sentinel HTTP-status errors above (e.g. a flagged challenge or a
// malformed request).
type APIError struct {
	Message string
}

func (e *APIError) Error() string { return "peak: " + e.Message }

func (c *Client) do(req *http.Request) (json.RawMessage, error) {
	req.Header.Set("X-API-Key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusPaymentRequired:
		return nil, ErrInsufficientBalance
	}
	var r apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	if !r.Success {
		msg := r.Error
		if msg == "" {
			msg = fmt.Sprintf("request failed (%d)", resp.StatusCode)
		}
		return nil, &APIError{Message: msg}
	}
	return r.Data, nil
}

func (c *Client) solve(ctx context.Context, payload map[string]string) (json.RawMessage, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/solve", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

// normalizeURL appends a trailing slash if the target page URL is missing
// one. The API rejects bare-path/query URLs outright and requires a
// trailing slash on the path (per https://peak.fo/docs/turnstile) — this
// is a common first-call mistake, so the client fixes it rather than
// forwarding a guaranteed-to-fail request.
func normalizeURL(url string) string {
	if strings.ContainsAny(url, "?#") {
		return url // don't rewrite query/fragment URLs; pass through as given.
	}
	if strings.HasSuffix(url, "/") {
		return url
	}
	return url + "/"
}

// SolveTurnstile solves a Cloudflare Turnstile challenge through your own
// proxy and returns the token for the cf-turnstile-response field. proxy
// must be in http://user:pass@host:port format; the API rejects the
// request without one — use SolveTurnstileProxyless to route through
// Peak's own pool instead.
func (c *Client) SolveTurnstile(ctx context.Context, pageURL, sitekey, proxy string) (string, error) {
	payload := map[string]string{
		"task_type": "turnstiletask",
		"url":       normalizeURL(pageURL),
		"sitekey":   sitekey,
		"proxy":     proxy,
	}
	return c.solveTurnstileToken(ctx, payload)
}

// SolveTurnstileProxyless solves a Cloudflare Turnstile challenge without
// supplying a proxy — Peak routes the request through its own pool. Priced
// the same per-1,000 as SolveTurnstile; see https://peak.fo/#pricing (the
// $1.20/1k figure on the docs page is stale as of 2026-10-03 — don't trust
// it for billing math).
func (c *Client) SolveTurnstileProxyless(ctx context.Context, pageURL, sitekey string) (string, error) {
	payload := map[string]string{
		"task_type": "TurnstileTaskProxyLess",
		"url":       normalizeURL(pageURL),
		"sitekey":   sitekey,
	}
	return c.solveTurnstileToken(ctx, payload)
}

func (c *Client) solveTurnstileToken(ctx context.Context, payload map[string]string) (string, error) {
	data, err := c.solve(ctx, payload)
	if err != nil {
		return "", err
	}
	var d struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return "", err
	}
	return d.Token, nil
}

// Cloudflare5sResult holds the clearance cookie and the matching user-agent
// the target site expects on follow-up requests.
type Cloudflare5sResult struct {
	Cookies    map[string]string `json:"cookies"`
	Headers    map[string]string `json:"headers"`
	Attributes map[string]string `json:"attributes"`
	CFRT       string            `json:"cf_rt"`
}

// SolveCloudflare5s clears the Cloudflare 5-second challenge. Use a sticky
// session proxy so the returned cf_clearance cookie stays valid for the
// requests you replay it on.
func (c *Client) SolveCloudflare5s(ctx context.Context, pageURL, proxy string) (*Cloudflare5sResult, error) {
	payload := map[string]string{"task_type": "cloudflare5stask", "url": pageURL}
	if proxy != "" {
		payload["proxy"] = proxy
	}
	data, err := c.solve(ctx, payload)
	if err != nil {
		return nil, err
	}
	var res Cloudflare5sResult
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// Balance holds account balance / plan status.
type Balance struct {
	Type    string  `json:"type"`
	Balance float64 `json:"balance"`
}

// GetBalance returns the account balance.
func (c *Client) GetBalance(ctx context.Context) (*Balance, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/balance", nil)
	if err != nil {
		return nil, err
	}
	data, err := c.do(req)
	if err != nil {
		return nil, err
	}
	var b Balance
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}
