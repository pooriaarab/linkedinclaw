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

func TestFetchConnections(t *testing.T) {
	fixturePath := filepath.Join("testdata", "connections.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture file: %v", err)
	}

	var expected connectionsResponse
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("failed to unmarshal expected connections: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/relationships/connections" {
			t.Errorf("expected path /relationships/connections, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient("dummy", "dummy")
	client.BaseURL = server.URL
	client.Limiter = nil

	connections, err := client.FetchConnections(context.Background())
	if err != nil {
		t.Fatalf("FetchConnections returned error: %v", err)
	}

	if len(connections) != len(expected.Elements) {
		t.Fatalf("expected %d connections, got %d", len(expected.Elements), len(connections))
	}

	for i, conn := range connections {
		exp := expected.Elements[i]
		if conn.Urn != exp.Urn {
			t.Errorf("[%d] expected Urn %q, got %q", i, exp.Urn, conn.Urn)
		}
		if conn.FirstName != exp.FirstName {
			t.Errorf("[%d] expected FirstName %q, got %q", i, exp.FirstName, conn.FirstName)
		}
		if conn.LastName != exp.LastName {
			t.Errorf("[%d] expected LastName %q, got %q", i, exp.LastName, conn.LastName)
		}
		if conn.Headline != exp.Headline {
			t.Errorf("[%d] expected Headline %q, got %q", i, exp.Headline, conn.Headline)
		}
		if conn.Company != exp.Company {
			t.Errorf("[%d] expected Company %q, got %q", i, exp.Company, conn.Company)
		}
		if conn.ConnectedAt != exp.ConnectedAt {
			t.Errorf("[%d] expected ConnectedAt %q, got %q", i, exp.ConnectedAt, conn.ConnectedAt)
		}
	}
}
