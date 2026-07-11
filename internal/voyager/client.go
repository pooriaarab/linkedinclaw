package voyager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// ErrDeferred is returned when a sync category is deferred due to rate limits or temporary errors.
var ErrDeferred = errors.New("voyager: sync category deferred due to rate limiting or temporary errors")

// Client is the Voyager API client.
type Client struct {
	client      *http.Client
	liAt        string
	jsessionID  string
	Limiter     *rate.Limiter
	BaseURL     string
	BackoffBase time.Duration
}

// NewClient creates a new Client with default configurations.
func NewClient(liAt, jsessionID string) *Client {
	return &Client{
		client:      &http.Client{Timeout: 30 * time.Second},
		liAt:        liAt,
		jsessionID:  jsessionID,
		Limiter:     rate.NewLimiter(rate.Every(time.Minute/20), 1), // default 20 requests/minute
		BaseURL:     "https://www.linkedin.com/voyager/api",
		BackoffBase: 1 * time.Second,
	}
}

// do makes an HTTP request to the Voyager API, applying rate-limiting, authorization headers, and retries with backoff.
func (c *Client) do(ctx context.Context, method, path string) (*http.Response, error) {
	const maxAttempts = 5

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// 1. Apply rate limiting
		if c.Limiter != nil {
			if err := c.Limiter.Wait(ctx); err != nil {
				return nil, err
			}
		}

		// 2. Prepare request
		url := c.BaseURL + path
		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// 3. Set standard auth headers
		// Cookie: li_at=<liAt>; JSESSIONID="<jsessionID>"
		// Csrf-Token: <jsessionID>
		cookieHeader := fmt.Sprintf("li_at=%s; JSESSIONID=\"%s\"", c.liAt, c.jsessionID)
		req.Header.Set("Cookie", cookieHeader)
		req.Header.Set("Csrf-Token", c.jsessionID)

		resp, err := c.client.Do(req)
		if err != nil {
			// On network-level errors, we could also retry, but the spec specifically says:
			// "on HTTP 429 or 999 response status, retry with exponential backoff... then return ErrDeferred"
			return nil, err
		}

		// 4. Handle rate limits or specific LinkedIn throttling errors (429 or 999)
		if resp.StatusCode == 429 || resp.StatusCode == 999 {
			resp.Body.Close()

			if attempt == maxAttempts {
				return nil, ErrDeferred
			}

			// Exponential backoff: base * 2^(attempt-1)
			backoff := c.BackoffBase * (1 << (attempt - 1))

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}

		return resp, nil
	}

	return nil, ErrDeferred
}

// isNonTrivialBody checks if a JSON body contains non-trivial content beyond standard empty definitions.
func isNonTrivialBody(body []byte) bool {
	s := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '"' || r == '\'' || r == '`' {
			return -1
		}
		return r
	}, string(body))
	s = strings.ToLower(s)
	return s != "" && s != "null" && s != "{}" && s != "[]" && s != "{elements:[]}" && s != "{elements:null}"
}
