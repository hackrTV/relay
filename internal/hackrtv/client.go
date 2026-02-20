package hackrtv

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"relay/internal/message"
)

type Client struct {
	wsURL   string
	token   string
	alias   string
	channel string
}

func NewClient(wsURL, token, alias, channel string) *Client {
	return &Client{
		wsURL:   wsURL,
		token:   token,
		alias:   alias,
		channel: channel,
	}
}

// ActionCable protocol messages
type cableMessage struct {
	Type       string          `json:"type,omitempty"`
	Message    json.RawMessage `json:"message,omitempty"`
	Identifier string          `json:"identifier,omitempty"`
	Command    string          `json:"command,omitempty"`
}

type channelIdentifier struct {
	Channel     string `json:"channel"`
	ChatChannel string `json:"chat_channel"`
}

// Packet data from hackr.tv
type packet struct {
	ID        int    `json:"id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	Dropped   bool   `json:"dropped"`
	GridHackr struct {
		ID        int    `json:"id"`
		HackrAlias string `json:"hackr_alias"`
		Role      string `json:"role"`
	} `json:"grid_hackr"`
}

type initialPacketsMessage struct {
	Type    string   `json:"type"`
	Packets []packet `json:"packets"`
}

type newPacketMessage struct {
	Type   string `json:"type"`
	Packet packet `json:"packet"`
}

func (c *Client) Connect(ctx context.Context, messages chan<- message.Message) error {
	// Build WebSocket URL with auth params
	u, err := url.Parse(c.wsURL)
	if err != nil {
		return fmt.Errorf("invalid websocket URL: %w", err)
	}
	q := u.Query()
	if c.token != "" {
		q.Set("token", c.token)
		q.Set("hackr_alias", c.alias)
	}
	u.RawQuery = q.Encode()

	// Set Origin header to match the server URL so ActionCable's
	// request forgery protection accepts the connection.
	origin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	if u.Scheme == "ws" {
		origin = fmt.Sprintf("http://%s", u.Host)
	} else if u.Scheme == "wss" {
		origin = fmt.Sprintf("https://%s", u.Host)
	}
	headers := map[string][]string{
		"Origin": {origin},
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return fmt.Errorf("failed to connect to hackr.tv: %w", err)
	}
	defer conn.Close()

	// Wait for ActionCable welcome message
	if err := c.waitForWelcome(conn); err != nil {
		return err
	}

	// Subscribe to LiveChatChannel
	if err := c.subscribe(conn); err != nil {
		return err
	}

	// Read loop
	readErr := make(chan error, 1)
	go func() {
		readErr <- c.readLoop(conn, messages)
	}()

	select {
	case <-ctx.Done():
		// Graceful close
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		return ctx.Err()
	case err := <-readErr:
		return err
	}
}

func (c *Client) waitForWelcome(conn *websocket.Conn) error {
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	var msg cableMessage
	if err := conn.ReadJSON(&msg); err != nil {
		return fmt.Errorf("failed to read welcome: %w", err)
	}
	if msg.Type != "welcome" {
		return fmt.Errorf("expected welcome, got %q", msg.Type)
	}
	return nil
}

func (c *Client) subscribe(conn *websocket.Conn) error {
	identifier := channelIdentifier{
		Channel:     "LiveChatChannel",
		ChatChannel: c.channel,
	}
	idJSON, err := json.Marshal(identifier)
	if err != nil {
		return fmt.Errorf("failed to marshal channel identifier: %w", err)
	}

	sub := cableMessage{
		Command:    "subscribe",
		Identifier: string(idJSON),
	}
	return conn.WriteJSON(sub)
}

// matchesSubscription checks if a cable message's identifier matches our
// subscription by comparing struct fields, avoiding brittle JSON string comparison.
func (c *Client) matchesSubscription(rawIdentifier string) bool {
	var id channelIdentifier
	if err := json.Unmarshal([]byte(rawIdentifier), &id); err != nil {
		return false
	}
	return id.Channel == "LiveChatChannel" && id.ChatChannel == c.channel
}

func (c *Client) readLoop(conn *websocket.Conn, messages chan<- message.Message) error {
	for {
		var raw cableMessage
		if err := conn.ReadJSON(&raw); err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		// Handle ActionCable protocol messages
		switch raw.Type {
		case "ping":
			continue
		case "confirm_subscription":
			continue
		case "reject_subscription":
			return fmt.Errorf("subscription rejected for channel %q", c.channel)
		case "disconnect":
			return fmt.Errorf("server disconnected: %s", string(raw.Message))
		}

		// Skip messages not for our subscription
		if !c.matchesSubscription(raw.Identifier) {
			continue
		}

		// Data message — parse the inner message
		if raw.Message == nil {
			continue
		}

		// Peek at the type field
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw.Message, &envelope); err != nil {
			continue
		}

		switch envelope.Type {
		case "initial_packets":
			var init initialPacketsMessage
			if err := json.Unmarshal(raw.Message, &init); err != nil {
				continue
			}
			for _, pkt := range init.Packets {
				if pkt.Dropped {
					continue
				}
				messages <- packetToMessage(pkt)
			}
		case "new_packet":
			var np newPacketMessage
			if err := json.Unmarshal(raw.Message, &np); err != nil {
				continue
			}
			if np.Packet.Dropped {
				continue
			}
			messages <- packetToMessage(np.Packet)
		}
	}
}

func packetToMessage(pkt packet) message.Message {
	ts, err := time.Parse(time.RFC3339, pkt.CreatedAt)
	if err != nil {
		ts = time.Now()
	}
	return message.Message{
		Platform:  message.HackrTV,
		Username:  pkt.GridHackr.HackrAlias,
		Timestamp: ts,
		Content:   pkt.Content,
	}
}
