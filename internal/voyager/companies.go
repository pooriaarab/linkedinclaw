// Field shapes are best-effort based on public linkedin-api conventions; verify against a real authenticated session before relying on this in production.

package voyager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Company represents a followed company on LinkedIn.
type Company struct {
	Urn        string `json:"urn"`
	Name       string `json:"name"`
	FollowedAt string `json:"followedAt"`
}

type companiesResponse struct {
	Elements []Company `json:"elements"`
}

// FetchFollowedCompanies retrieves followed companies for the authenticated user from the Voyager API.
func (c *Client) FetchFollowedCompanies(ctx context.Context) ([]Company, error) {
	resp, err := c.do(ctx, "GET", "/relationships/followed-companies")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching followed companies", resp.StatusCode)
	}

	var res companiesResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode companies JSON: %w", err)
	}

	return res.Elements, nil
}
