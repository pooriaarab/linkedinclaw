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

func TestFetchFollowedCompanies(t *testing.T) {
	fixturePath := filepath.Join("testdata", "companies.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture file: %v", err)
	}

	var expected companiesResponse
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("failed to unmarshal expected companies: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/relationships/followed-companies" {
			t.Errorf("expected path /relationships/followed-companies, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient("dummy", "dummy")
	client.BaseURL = server.URL
	client.Limiter = nil

	companies, err := client.FetchFollowedCompanies(context.Background())
	if err != nil {
		t.Fatalf("FetchFollowedCompanies returned error: %v", err)
	}

	if len(companies) != len(expected.Elements) {
		t.Fatalf("expected %d companies, got %d", len(expected.Elements), len(companies))
	}

	for i, company := range companies {
		exp := expected.Elements[i]
		if company.Urn != exp.Urn {
			t.Errorf("[%d] expected Urn %q, got %q", i, exp.Urn, company.Urn)
		}
		if company.Name != exp.Name {
			t.Errorf("[%d] expected Name %q, got %q", i, exp.Name, company.Name)
		}
		if company.FollowedAt != exp.FollowedAt {
			t.Errorf("[%d] expected FollowedAt %q, got %q", i, exp.FollowedAt, company.FollowedAt)
		}
	}
}
