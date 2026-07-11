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

// Conversation represents a messaging conversation.
type Conversation struct {
	Urn            string `json:"urn"`
	Participants   string `json:"participants"`
	LastActivityAt string `json:"lastActivityAt"`
}

type conversationsResponse struct {
	Elements []Conversation `json:"elements"`
}

// Message represents a direct message event within a conversation.
type Message struct {
	Urn             string `json:"urn"`
	ConversationUrn string `json:"conversationUrn"`
	SenderUrn       string `json:"senderUrn"`
	Body            string `json:"body"`
	SentAt          string `json:"sentAt"`
}

type messagesResponse struct {
	Elements []Message `json:"elements"`
}

// FetchConversations retrieves conversations from the Voyager API.
func (c *Client) FetchConversations(ctx context.Context) ([]Conversation, error) {
	resp, err := c.do(ctx, "GET", "/messaging/conversations")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching conversations", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read conversations body: %w", err)
	}

	var res conversationsResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("failed to decode conversations JSON: %w", err)
	}

	if len(res.Elements) == 0 && isNonTrivialBody(bodyBytes) {
		fmt.Fprintln(os.Stderr, "warning: parsed 0 conversations from a non-empty response -- field-shape assumptions for this endpoint are unverified")
	}

	return res.Elements, nil
}

// FetchMessages retrieves message events for a given conversation from the Voyager API.
func (c *Client) FetchMessages(ctx context.Context, conversationUrn string) ([]Message, error) {
	path := fmt.Sprintf("/messaging/conversations/%s/events", conversationUrn)
	resp, err := c.do(ctx, "GET", path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching messages", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read messages body: %w", err)
	}

	var res messagesResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("failed to decode messages JSON: %w", err)
	}

	if len(res.Elements) == 0 && isNonTrivialBody(bodyBytes) {
		fmt.Fprintln(os.Stderr, "warning: parsed 0 messages from a non-empty response -- field-shape assumptions for this endpoint are unverified")
	}

	return res.Elements, nil
}
