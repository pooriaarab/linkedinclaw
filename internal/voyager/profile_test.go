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

func TestFetchProfile(t *testing.T) {
	fixturePath := filepath.Join("testdata", "profile.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture file: %v", err)
	}

	var expected Profile
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("failed to unmarshal expected profile: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity/profiles/me" {
			t.Errorf("expected path /identity/profiles/me, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient("dummy", "dummy")
	client.BaseURL = server.URL
	client.Limiter = nil

	profile, err := client.FetchProfile(context.Background())
	if err != nil {
		t.Fatalf("FetchProfile returned error: %v", err)
	}

	if profile.Urn != expected.Urn {
		t.Errorf("expected Urn %q, got %q", expected.Urn, profile.Urn)
	}
	if profile.FirstName != expected.FirstName {
		t.Errorf("expected FirstName %q, got %q", expected.FirstName, profile.FirstName)
	}
	if profile.LastName != expected.LastName {
		t.Errorf("expected LastName %q, got %q", expected.LastName, profile.LastName)
	}
	if profile.Headline != expected.Headline {
		t.Errorf("expected Headline %q, got %q", expected.Headline, profile.Headline)
	}
}
