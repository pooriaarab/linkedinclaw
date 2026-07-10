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

func TestFetchSavedPosts(t *testing.T) {
	fixturePath := filepath.Join("testdata", "saved.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture file: %v", err)
	}

	var expected savedPostsResponse
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("failed to unmarshal expected saved posts: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity/profiles/me/saved-posts" {
			t.Errorf("expected path /identity/profiles/me/saved-posts, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient("dummy", "dummy")
	client.BaseURL = server.URL
	client.Limiter = nil

	posts, err := client.FetchSavedPosts(context.Background())
	if err != nil {
		t.Fatalf("FetchSavedPosts returned error: %v", err)
	}

	if len(posts) != len(expected.Elements) {
		t.Fatalf("expected %d saved posts, got %d", len(expected.Elements), len(posts))
	}

	for i, post := range posts {
		exp := expected.Elements[i]
		if post.Urn != exp.Urn {
			t.Errorf("[%d] expected Urn %q, got %q", i, exp.Urn, post.Urn)
		}
		if post.Author != exp.Author {
			t.Errorf("[%d] expected Author %q, got %q", i, exp.Author, post.Author)
		}
		if post.Body != exp.Body {
			t.Errorf("[%d] expected Body %q, got %q", i, exp.Body, post.Body)
		}
		if post.SavedAt != exp.SavedAt {
			t.Errorf("[%d] expected SavedAt %q, got %q", i, exp.SavedAt, post.SavedAt)
		}
	}
}
