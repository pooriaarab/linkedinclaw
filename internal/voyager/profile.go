// This endpoint (path and shape) was verified against a real account on 2026-07-10,
// unlike the other 5 voyager fetchers which remain best-effort guesses. The old path
// "/identity/profiles/me" returns HTTP 410 Gone against the live API; "/me" is the
// working replacement.

package voyager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Profile represents the envelope of the authenticated user's profile.
type Profile struct {
	PlainID           any         `json:"plainId"`
	MiniProfile       MiniProfile `json:"miniProfile"`
	PremiumSubscriber any         `json:"premiumSubscriber"`
}

// MiniProfile contains the nested identity fields under miniProfile.
type MiniProfile struct {
	EntityUrn        string `json:"entityUrn"`
	FirstName        string `json:"firstName"`
	LastName         string `json:"lastName"`
	Occupation       string `json:"occupation"`
	PublicIdentifier string `json:"publicIdentifier"`
}

// FetchProfile retrieves the profile of the authenticated user.
func (c *Client) FetchProfile(ctx context.Context) (Profile, error) {
	resp, err := c.do(ctx, "GET", "/me")
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
