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

// SavedPost represents a bookmarked or saved post on LinkedIn.
type SavedPost struct {
	Urn     string `json:"urn"`
	Author  string `json:"author"`
	Body    string `json:"body"`
	SavedAt string `json:"savedAt"`
}

type savedPostsResponse struct {
	Elements []SavedPost `json:"elements"`
}

// FetchSavedPosts retrieves saved/bookmarked posts for the authenticated user from the Voyager API.
func (c *Client) FetchSavedPosts(ctx context.Context) ([]SavedPost, error) {
	resp, err := c.do(ctx, "GET", "/identity/profiles/me/saved-posts")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching saved posts", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read saved posts body: %w", err)
	}

	var res savedPostsResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("failed to decode saved posts JSON: %w", err)
	}

	if len(res.Elements) == 0 && isNonTrivialBody(bodyBytes) {
		fmt.Fprintln(os.Stderr, "warning: parsed 0 saved posts from a non-empty response -- field-shape assumptions for this endpoint are unverified")
	}

	return res.Elements, nil
}
