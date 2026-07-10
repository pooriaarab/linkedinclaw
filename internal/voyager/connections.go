// Field shapes are best-effort based on public linkedin-api conventions; verify against a real authenticated session before relying on this in production.

package voyager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	var res connectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode connections JSON: %w", err)
	}

	return res.Elements, nil
}
