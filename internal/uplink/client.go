package uplink

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"relay/internal/message"
)

// ErrRateLimit is returned when the Uplink API responds with 429.
var ErrRateLimit = errors.New("uplink: rate limited")

// Client sends chat messages to hackr.tv via the Admin Uplink API.
type Client struct {
	baseURL string
	token   string
	channel string
	http    *http.Client
}

// NewClient creates an Uplink API client.
// wsURL is the ActionCable WebSocket URL (e.g. wss://hackr.tv/cable).
// token is the per-hackr API token, alias is the hackr alias.
// channel is the chat channel slug.
func NewClient(wsURL, token, alias, channel string) (*Client, error) {
	base, err := deriveBaseURL(wsURL)
	if err != nil {
		return nil, fmt.Errorf("uplink: %w", err)
	}

	return &Client{
		baseURL: base,
		token:   alias + ":" + token,
		channel: channel,
		http:    &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// deriveBaseURL converts a WebSocket URL to an HTTP base URL.
// ws://host:port/cable → http://host:port
// wss://host:port/cable → https://host:port
func deriveBaseURL(wsURL string) (string, error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return "", fmt.Errorf("invalid WebSocket URL: %w", err)
	}

	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	default:
		return "", fmt.Errorf("unexpected scheme %q, expected ws or wss", u.Scheme)
	}

	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

// FormatContent formats a message for the Uplink API.
// Format: "[TTV] nightbot: !commands" — truncated to 512 chars.
func FormatContent(msg message.Message) string {
	s := fmt.Sprintf("[%s] %s: %s", msg.Platform, msg.Username, msg.Content)
	if len(s) > 512 {
		s = s[:512]
	}
	return s
}

type sendPayload struct {
	ChannelSlug string `json:"channel_slug"`
	Content     string `json:"content"`
}

// Send posts a single message to the Uplink API.
func (c *Client) Send(ctx context.Context, msg message.Message) error {
	body, err := json.Marshal(sendPayload{
		ChannelSlug: c.channel,
		Content:     FormatContent(msg),
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/admin/uplink/send_packet", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusCreated:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return ErrRateLimit
	default:
		return fmt.Errorf("uplink: unexpected status %d", resp.StatusCode)
	}
}

// Run reads messages from the channel and sends each to the Uplink API.
// On rate limiting it backs off for 2 seconds. Stops when ctx is cancelled
// or the channel is closed.
func (c *Client) Run(ctx context.Context, messages <-chan message.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messages:
			if !ok {
				return
			}
			err := c.Send(ctx, msg)
			if err == nil {
				continue
			}
			if errors.Is(err, ErrRateLimit) {
				fmt.Fprintln(os.Stderr, "Uplink rate limited, backing off 2s")
				select {
				case <-time.After(2 * time.Second):
				case <-ctx.Done():
					return
				}
				continue
			}
			if ctx.Err() != nil {
				return
			}
			fmt.Fprintf(os.Stderr, "Uplink send error: %v\n", err)
		}
	}
}
