package uplink

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"relay/internal/message"
)

func TestDeriveBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		wsURL   string
		want    string
		wantErr bool
	}{
		{
			name:  "ws to http",
			wsURL: "ws://localhost:3000/cable",
			want:  "http://localhost:3000",
		},
		{
			name:  "wss to https",
			wsURL: "wss://hackr.tv/cable",
			want:  "https://hackr.tv",
		},
		{
			name:  "ws with port and path stripped",
			wsURL: "ws://127.0.0.1:8080/cable?token=abc",
			want:  "http://127.0.0.1:8080",
		},
		{
			name:    "http scheme rejected",
			wsURL:   "http://localhost/cable",
			wantErr: true,
		},
		{
			name:    "empty string",
			wsURL:   "://bad",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := deriveBaseURL(tt.wsURL)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("deriveBaseURL(%q) = %q, want %q", tt.wsURL, got, tt.want)
			}
		})
	}
}

func TestFormatContent(t *testing.T) {
	tests := []struct {
		name string
		msg  message.Message
		want string
	}{
		{
			name: "twitch message",
			msg: message.Message{
				Platform: message.Twitch,
				Username: "nightbot",
				Content:  "!commands",
			},
			want: "[TTV] nightbot: !commands",
		},
		{
			name: "youtube message",
			msg: message.Message{
				Platform: message.YouTube,
				Username: "viewer",
				Content:  "hello world",
			},
			want: "[YT_] viewer: hello world",
		},
		{
			name: "truncation at 512 chars",
			msg: message.Message{
				Platform: message.Twitch,
				Username: "user",
				Content:  strings.Repeat("a", 600),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatContent(tt.msg)
			if tt.name == "truncation at 512 chars" {
				if len(got) != 512 {
					t.Errorf("len = %d, want 512", len(got))
				}
				return
			}
			if got != tt.want {
				t.Errorf("FormatContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and path
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/admin/uplink/send_packet" {
			t.Errorf("path = %q, want /api/admin/uplink/send_packet", r.URL.Path)
		}

		// Verify auth header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer XERAEN:secret123" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer XERAEN:secret123")
		}

		// Verify content type
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		// Verify body
		body, _ := io.ReadAll(r.Body)
		var payload sendPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("invalid JSON body: %v", err)
		}
		if payload.ChannelSlug != "live" {
			t.Errorf("channel_slug = %q, want %q", payload.ChannelSlug, "live")
		}
		if !strings.Contains(payload.Content, "[TTV]") {
			t.Errorf("content missing [TTV] prefix: %q", payload.Content)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		token:   "XERAEN:secret123",
		channel: "live",
		http:    server.Client(),
	}

	msg := message.Message{
		Platform:  message.Twitch,
		Username:  "nightbot",
		Content:   "!commands",
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
}

func TestSendRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		token:   "a:b",
		channel: "live",
		http:    server.Client(),
	}

	err := client.Send(context.Background(), message.Message{
		Platform: message.Twitch,
		Username: "user",
		Content:  "test",
	})
	if err != ErrRateLimit {
		t.Errorf("Send() error = %v, want ErrRateLimit", err)
	}
}

func TestSendValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		token:   "a:b",
		channel: "live",
		http:    server.Client(),
	}

	err := client.Send(context.Background(), message.Message{
		Platform: message.Twitch,
		Username: "user",
		Content:  "test",
	})
	if err == nil {
		t.Fatal("Send() expected error for 422")
	}
	if !strings.Contains(err.Error(), "422") {
		t.Errorf("error should mention 422: %v", err)
	}
}

func TestRunSkipsHackrTV(t *testing.T) {
	var hitCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		token:   "a:b",
		channel: "live",
		http:    server.Client(),
	}

	// Feed messages through the fan-out dispatcher (same logic as main.go)
	// to verify only non-HTV messages reach the uplink
	uplinkCh := make(chan message.Message, 10)
	msgs := []message.Message{
		{Platform: message.Twitch, Username: "ttv_user", Content: "hello"},
		{Platform: message.HackrTV, Username: "htv_user", Content: "should skip"},
		{Platform: message.YouTube, Username: "yt_user", Content: "world"},
		{Platform: message.HackrTV, Username: "htv_user2", Content: "also skip"},
	}

	// Simulate the fan-out filter from main.go
	for _, msg := range msgs {
		if msg.Platform != message.HackrTV {
			uplinkCh <- msg
		}
	}
	close(uplinkCh)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client.Run(ctx, uplinkCh)

	if got := hitCount.Load(); got != 2 {
		t.Errorf("expected 2 requests (TTV + YT), got %d", got)
	}
}
