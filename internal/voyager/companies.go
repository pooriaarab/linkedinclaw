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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read followed companies body: %w", err)
	}

	var res companiesResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("failed to decode companies JSON: %w", err)
	}

	if len(res.Elements) == 0 && isNonTrivialBody(bodyBytes) {
		fmt.Fprintln(os.Stderr, "warning: parsed 0 followed companies from a non-empty response -- field-shape assumptions for this endpoint are unverified")
	}

	return res.Elements, nil
}
