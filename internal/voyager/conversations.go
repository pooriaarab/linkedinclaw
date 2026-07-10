// Field shapes are best-effort based on public linkedin-api conventions; verify against a real authenticated session before relying on this in production.

package voyager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	var res conversationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode conversations JSON: %w", err)
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

	var res messagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode messages JSON: %w", err)
	}

	return res.Elements, nil
}
