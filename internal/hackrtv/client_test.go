package hackrtv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"relay/internal/message"
)

func TestMatchesSubscription(t *testing.T) {
	c := NewClient("ws://localhost/cable", "", "", "main")

	tests := []struct {
		name       string
		identifier string
		want       bool
	}{
		{
			name:       "exact match same key order",
			identifier: `{"channel":"LiveChatChannel","chat_channel":"main"}`,
			want:       true,
		},
		{
			name:       "reversed key order",
			identifier: `{"chat_channel":"main","channel":"LiveChatChannel"}`,
			want:       true,
		},
		{
			name:       "extra whitespace",
			identifier: `{ "channel" : "LiveChatChannel" , "chat_channel" : "main" }`,
			want:       true,
		},
		{
			name:       "wrong channel slug",
			identifier: `{"channel":"LiveChatChannel","chat_channel":"other"}`,
			want:       false,
		},
		{
			name:       "wrong channel class",
			identifier: `{"channel":"OtherChannel","chat_channel":"main"}`,
			want:       false,
		},
		{
			name:       "invalid JSON",
			identifier: `not json`,
			want:       false,
		},
		{
			name:       "empty string",
			identifier: "",
			want:       false,
		},
		{
			name:       "extra fields still match",
			identifier: `{"channel":"LiveChatChannel","chat_channel":"main","extra":"field"}`,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.matchesSubscription(tt.identifier)
			if got != tt.want {
				t.Errorf("matchesSubscription(%q) = %v, want %v", tt.identifier, got, tt.want)
			}
		})
	}
}

func TestPacketToMessage(t *testing.T) {
	pkt := packet{
		ID:        42,
		Content:   "hello grid",
		CreatedAt: "2025-06-15T10:30:00Z",
		Dropped:   false,
	}
	pkt.GridHackr.ID = 1
	pkt.GridHackr.HackrAlias = "xeraen"
	pkt.GridHackr.Role = "admin"

	msg := packetToMessage(pkt)

	if msg.Platform != message.HackrTV {
		t.Errorf("Platform = %v, want HackrTV", msg.Platform)
	}
	if msg.Username != "xeraen" {
		t.Errorf("Username = %q, want %q", msg.Username, "xeraen")
	}
	if msg.Content != "hello grid" {
		t.Errorf("Content = %q, want %q", msg.Content, "hello grid")
	}
	expectedTime := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	if !msg.Timestamp.Equal(expectedTime) {
		t.Errorf("Timestamp = %v, want %v", msg.Timestamp, expectedTime)
	}
}

func TestPacketToMessageInvalidTimestamp(t *testing.T) {
	pkt := packet{
		Content:   "test",
		CreatedAt: "not-a-date",
	}
	pkt.GridHackr.HackrAlias = "user"

	before := time.Now()
	msg := packetToMessage(pkt)
	after := time.Now()

	if msg.Timestamp.Before(before) || msg.Timestamp.After(after) {
		t.Errorf("expected fallback to time.Now(), got %v", msg.Timestamp)
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("ws://localhost/cable", "secret", "relay", "main")

	if c.wsURL != "ws://localhost/cable" {
		t.Errorf("wsURL = %q", c.wsURL)
	}
	if c.token != "secret" {
		t.Errorf("token = %q", c.token)
	}
	if c.alias != "relay" {
		t.Errorf("alias = %q", c.alias)
	}
	if c.channel != "main" {
		t.Errorf("channel = %q", c.channel)
	}
}

// Mock ActionCable WebSocket server for integration tests
var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func TestConnectFullProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth params are passed
		if r.URL.Query().Get("token") != "test_token" {
			t.Errorf("expected token param, got %q", r.URL.Query().Get("token"))
		}
		if r.URL.Query().Get("hackr_alias") != "relay" {
			t.Errorf("expected hackr_alias param, got %q", r.URL.Query().Get("hackr_alias"))
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		// Send welcome
		conn.WriteJSON(cableMessage{Type: "welcome"})

		// Read subscribe command
		var sub cableMessage
		if err := conn.ReadJSON(&sub); err != nil {
			return
		}
		if sub.Command != "subscribe" {
			t.Errorf("expected subscribe command, got %q", sub.Command)
		}

		// Confirm subscription
		conn.WriteJSON(cableMessage{Type: "confirm_subscription", Identifier: sub.Identifier})

		// Send a ping (should be ignored)
		conn.WriteJSON(cableMessage{Type: "ping"})

		// Send initial_packets
		initPayload, _ := json.Marshal(initialPacketsMessage{
			Type: "initial_packets",
			Packets: []packet{
				{ID: 1, Content: "old dropped msg", CreatedAt: "2025-01-01T00:00:00Z", Dropped: true,
					GridHackr: struct {
						ID         int    `json:"id"`
						HackrAlias string `json:"hackr_alias"`
						Role       string `json:"role"`
					}{1, "someone", "operative"}},
				{ID: 2, Content: "old msg", CreatedAt: "2025-01-01T00:00:01Z",
					GridHackr: struct {
						ID         int    `json:"id"`
						HackrAlias string `json:"hackr_alias"`
						Role       string `json:"role"`
					}{2, "hackr1", "operative"}},
			},
		})
		conn.WriteJSON(cableMessage{
			Identifier: sub.Identifier,
			Message:    json.RawMessage(initPayload),
		})

		// Send a new_packet
		npPayload, _ := json.Marshal(newPacketMessage{
			Type: "new_packet",
			Packet: packet{
				ID: 3, Content: "live msg", CreatedAt: "2025-01-01T00:01:00Z",
				GridHackr: struct {
					ID         int    `json:"id"`
					HackrAlias string `json:"hackr_alias"`
					Role       string `json:"role"`
				}{3, "hackr2", "admin"},
			},
		})
		conn.WriteJSON(cableMessage{
			Identifier: sub.Identifier,
			Message:    json.RawMessage(npPayload),
		})

		// Send a dropped new_packet (should be filtered)
		droppedPayload, _ := json.Marshal(newPacketMessage{
			Type: "new_packet",
			Packet: packet{
				ID: 4, Content: "dropped live", CreatedAt: "2025-01-01T00:02:00Z", Dropped: true,
				GridHackr: struct {
					ID         int    `json:"id"`
					HackrAlias string `json:"hackr_alias"`
					Role       string `json:"role"`
				}{4, "hackr3", "operative"},
			},
		})
		conn.WriteJSON(cableMessage{
			Identifier: sub.Identifier,
			Message:    json.RawMessage(droppedPayload),
		})

		// Close after sending all messages
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(wsURL, "test_token", "relay", "main")

	messages := make(chan message.Message, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect will return when server closes the connection
	client.Connect(ctx, messages)
	close(messages)

	var received []message.Message
	for msg := range messages {
		received = append(received, msg)
	}

	// Should get 2 messages: 1 from initial (dropped filtered) + 1 new (dropped filtered)
	if len(received) != 2 {
		t.Fatalf("expected 2 messages, got %d: %+v", len(received), received)
	}

	if received[0].Content != "old msg" {
		t.Errorf("msg[0].Content = %q, want %q", received[0].Content, "old msg")
	}
	if received[0].Username != "hackr1" {
		t.Errorf("msg[0].Username = %q, want %q", received[0].Username, "hackr1")
	}

	if received[1].Content != "live msg" {
		t.Errorf("msg[1].Content = %q, want %q", received[1].Content, "live msg")
	}
	if received[1].Username != "hackr2" {
		t.Errorf("msg[1].Username = %q, want %q", received[1].Username, "hackr2")
	}

	for _, msg := range received {
		if msg.Platform != message.HackrTV {
			t.Errorf("expected HackrTV platform, got %v", msg.Platform)
		}
	}
}

func TestConnectNoToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no auth params when token is empty
		if r.URL.Query().Get("token") != "" {
			t.Errorf("expected no token param, got %q", r.URL.Query().Get("token"))
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		conn.WriteJSON(cableMessage{Type: "welcome"})

		var sub cableMessage
		conn.ReadJSON(&sub)
		conn.WriteJSON(cableMessage{Type: "confirm_subscription", Identifier: sub.Identifier})

		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(wsURL, "", "relay", "main")

	messages := make(chan message.Message, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client.Connect(ctx, messages)
	// No assertion needed — just verify no panic/crash with empty token
}

func TestConnectRejectedSubscription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		conn.WriteJSON(cableMessage{Type: "welcome"})

		var sub cableMessage
		conn.ReadJSON(&sub)

		// Reject the subscription
		conn.WriteJSON(cableMessage{Type: "reject_subscription", Identifier: sub.Identifier})
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(wsURL, "token", "relay", "main")

	messages := make(chan message.Message, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx, messages)
	if err == nil || !strings.Contains(err.Error(), "subscription rejected") {
		t.Errorf("expected subscription rejected error, got: %v", err)
	}
}

func TestConnectBadWelcome(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send wrong type instead of welcome
		conn.WriteJSON(cableMessage{Type: "disconnect"})
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(wsURL, "", "", "main")

	messages := make(chan message.Message, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx, messages)
	if err == nil || !strings.Contains(err.Error(), "expected welcome") {
		t.Errorf("expected welcome error, got: %v", err)
	}
}
