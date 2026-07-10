// Field shapes are best-effort based on public linkedin-api conventions; verify against a real authenticated session before relying on this in production.

package voyager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	var res postsResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode posts JSON: %w", err)
	}

	return res.Elements, nil
}
