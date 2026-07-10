package voyager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFetchOwnPosts(t *testing.T) {
	fixturePath := filepath.Join("testdata", "posts.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture file: %v", err)
	}

	var expected postsResponse
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("failed to unmarshal expected posts: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity/profiles/me/posts" {
			t.Errorf("expected path /identity/profiles/me/posts, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient("dummy", "dummy")
	client.BaseURL = server.URL
	client.Limiter = nil

	posts, err := client.FetchOwnPosts(context.Background())
	if err != nil {
		t.Fatalf("FetchOwnPosts returned error: %v", err)
	}

	if len(posts) != len(expected.Elements) {
		t.Fatalf("expected %d posts, got %d", len(expected.Elements), len(posts))
	}

	for i, post := range posts {
		exp := expected.Elements[i]
		if post.Urn != exp.Urn {
			t.Errorf("[%d] expected Urn %q, got %q", i, exp.Urn, post.Urn)
		}
		if post.Body != exp.Body {
			t.Errorf("[%d] expected Body %q, got %q", i, exp.Body, post.Body)
		}
		if post.PostedAt != exp.PostedAt {
			t.Errorf("[%d] expected PostedAt %q, got %q", i, exp.PostedAt, post.PostedAt)
		}
		if post.LikeCount != exp.LikeCount {
			t.Errorf("[%d] expected LikeCount %d, got %d", i, exp.LikeCount, post.LikeCount)
		}
		if post.CommentCount != exp.CommentCount {
			t.Errorf("[%d] expected CommentCount %d, got %d", i, exp.CommentCount, post.CommentCount)
		}
	}
}
