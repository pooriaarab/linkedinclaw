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

func TestFetchConversations(t *testing.T) {
	fixturePath := filepath.Join("testdata", "conversations.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture file: %v", err)
	}

	var expected conversationsResponse
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("failed to unmarshal expected conversations: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messaging/conversations" {
			t.Errorf("expected path /messaging/conversations, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient("dummy", "dummy")
	client.BaseURL = server.URL
	client.Limiter = nil

	conversations, err := client.FetchConversations(context.Background())
	if err != nil {
		t.Fatalf("FetchConversations returned error: %v", err)
	}

	if len(conversations) != len(expected.Elements) {
		t.Fatalf("expected %d conversations, got %d", len(expected.Elements), len(conversations))
	}

	for i, conv := range conversations {
		exp := expected.Elements[i]
		if conv.Urn != exp.Urn {
			t.Errorf("[%d] expected Urn %q, got %q", i, exp.Urn, conv.Urn)
		}
		if conv.Participants != exp.Participants {
			t.Errorf("[%d] expected Participants %q, got %q", i, exp.Participants, conv.Participants)
		}
		if conv.LastActivityAt != exp.LastActivityAt {
			t.Errorf("[%d] expected LastActivityAt %q, got %q", i, exp.LastActivityAt, conv.LastActivityAt)
		}
	}
}

func TestFetchMessages(t *testing.T) {
	fixturePath := filepath.Join("testdata", "messages.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture file: %v", err)
	}

	var expected messagesResponse
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("failed to unmarshal expected messages: %v", err)
	}

	targetConv := "urn:li:fs_conversation:111"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/messaging/conversations/" + targetConv + "/events"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %q, got %q", expectedPath, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient("dummy", "dummy")
	client.BaseURL = server.URL
	client.Limiter = nil

	messages, err := client.FetchMessages(context.Background(), targetConv)
	if err != nil {
		t.Fatalf("FetchMessages returned error: %v", err)
	}

	if len(messages) != len(expected.Elements) {
		t.Fatalf("expected %d messages, got %d", len(expected.Elements), len(messages))
	}

	for i, msg := range messages {
		exp := expected.Elements[i]
		if msg.Urn != exp.Urn {
			t.Errorf("[%d] expected Urn %q, got %q", i, exp.Urn, msg.Urn)
		}
		if msg.ConversationUrn != exp.ConversationUrn {
			t.Errorf("[%d] expected ConversationUrn %q, got %q", i, exp.ConversationUrn, msg.ConversationUrn)
		}
		if msg.SenderUrn != exp.SenderUrn {
			t.Errorf("[%d] expected SenderUrn %q, got %q", i, exp.SenderUrn, msg.SenderUrn)
		}
		if msg.Body != exp.Body {
			t.Errorf("[%d] expected Body %q, got %q", i, exp.Body, msg.Body)
		}
		if msg.SentAt != exp.SentAt {
			t.Errorf("[%d] expected SentAt %q, got %q", i, exp.SentAt, msg.SentAt)
		}
	}
}
