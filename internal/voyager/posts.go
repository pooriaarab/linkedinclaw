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

// Post represents an authored post on LinkedIn.
type Post struct {
	Urn          string `json:"urn"`
	Body         string `json:"body"`
	PostedAt     string `json:"postedAt"`
	LikeCount    int    `json:"likeCount"`
	CommentCount int    `json:"commentCount"`
}

type postsResponse struct {
	Elements []Post `json:"elements"`
}

// FetchOwnPosts retrieves authored posts for the authenticated user from the Voyager API.
func (c *Client) FetchOwnPosts(ctx context.Context) ([]Post, error) {
	resp, err := c.do(ctx, "GET", "/identity/profiles/me/posts")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching own posts", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read posts body: %w", err)
	}

	var res postsResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("failed to decode posts JSON: %w", err)
	}

	if len(res.Elements) == 0 && isNonTrivialBody(bodyBytes) {
		fmt.Fprintln(os.Stderr, "warning: parsed 0 posts from a non-empty response -- field-shape assumptions for this endpoint are unverified")
	}

	return res.Elements, nil
}
