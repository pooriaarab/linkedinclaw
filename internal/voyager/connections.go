// Field shapes are best-effort based on public linkedin-api conventions; verify against a real authenticated session before relying on this in production.

package voyager

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Connection represents a first-degree connection on LinkedIn.
type Connection struct {
	Urn         string `json:"urn"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Headline    string `json:"headline"`
	Company     string `json:"company"`
	ConnectedAt string `json:"connectedAt"`
}

type connectionsResponse struct {
	Elements []Connection `json:"elements"`
}

// FetchConnections retrieves first-degree connections from the Voyager API.
func (c *Client) FetchConnections(ctx context.Context) ([]Connection, error) {
	resp, err := c.do(ctx, "GET", "/relationships/connections")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching connections", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read connections body: %w", err)
	}

	var res connectionsResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("failed to decode connections JSON: %w", err)
	}

	if len(res.Elements) == 0 && isNonTrivialBody(bodyBytes) {
		fmt.Fprintln(os.Stderr, "warning: parsed 0 connections from a non-empty response -- field-shape assumptions for this endpoint are unverified")
	}

	return res.Elements, nil
}
