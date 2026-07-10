package voyager

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_Headers(t *testing.T) {
	liAt := "dummy-li-at"
	jsessionID := "dummy-jsessionid"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie := r.Header.Get("Cookie")
		expectedCookie := `li_at=dummy-li-at; JSESSIONID="dummy-jsessionid"`
		if cookie != expectedCookie {
			t.Errorf("expected Cookie header %q, got %q", expectedCookie, cookie)
		}

		csrf := r.Header.Get("Csrf-Token")
		if csrf != jsessionID {
			t.Errorf("expected Csrf-Token %q, got %q", jsessionID, csrf)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(liAt, jsessionID)
	client.BaseURL = server.URL // override base URL for the test

	ctx := context.Background()
	resp, err := client.do(ctx, "GET", "/test-endpoint")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %d", resp.StatusCode)
	}
}

func TestClient_RetriesAndDeferred(t *testing.T) {
	liAt := "dummy-li-at"
	jsessionID := "dummy-jsessionid"

	var attemptCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		// Return 429 on early requests, and 999 on the final ones to test both throttling codes
		if attemptCount%2 == 1 {
			w.WriteHeader(http.StatusTooManyRequests) // 429
		} else {
			w.WriteHeader(999) // 999
		}
	}))
	defer server.Close()

	client := NewClient(liAt, jsessionID)
	client.BaseURL = server.URL
	client.backoffBase = 1 * time.Millisecond // very short backoff for testing
	// Bypass or configure limiter so we don't delay the test
	client.Limiter = nil

	ctx := context.Background()
	resp, err := client.do(ctx, "GET", "/throttled-endpoint")
	if !errors.Is(err, ErrDeferred) {
		t.Fatalf("expected ErrDeferred, got: %v", err)
	}
	if resp != nil {
		t.Errorf("expected response to be nil on ErrDeferred, got: %+v", resp)
	}

	const expectedAttempts = 5
	if attemptCount != expectedAttempts {
		t.Errorf("expected %d total attempts, got %d", expectedAttempts, attemptCount)
	}
}
