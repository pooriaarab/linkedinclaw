// Field shapes are best-effort based on public linkedin-api conventions; verify against a real authenticated session before relying on this in production.

package voyager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Profile represents a LinkedIn mini profile returned from the Voyager API.
type Profile struct {
	Urn       string `json:"urn"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Headline  string `json:"headline"`
}

// FetchProfile retrieves the profile of the authenticated user.
func (c *Client) FetchProfile(ctx context.Context) (Profile, error) {
	resp, err := c.do(ctx, "GET", "/identity/profiles/me")
	if err != nil {
		return Profile{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Profile{}, fmt.Errorf("unexpected status code %d fetching profile", resp.StatusCode)
	}

	var p Profile
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return Profile{}, fmt.Errorf("failed to decode profile JSON: %w", err)
	}

	return p, nil
}
