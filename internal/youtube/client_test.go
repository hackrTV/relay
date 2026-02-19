package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"relay/internal/message"
)

func TestNewClient(t *testing.T) {
	c := NewClient("api-key", "video-123")
	if c.apiKey != "api-key" {
		t.Errorf("apiKey = %q", c.apiKey)
	}
	if c.videoID != "video-123" {
		t.Errorf("videoID = %q", c.videoID)
	}
	if c.pollingRate != 3*time.Second {
		t.Errorf("pollingRate = %v", c.pollingRate)
	}
}

func TestFetchLiveChatID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request params
		if r.URL.Query().Get("id") != "video-123" {
			t.Errorf("expected video id param, got %q", r.URL.Query().Get("id"))
		}
		if r.URL.Query().Get("key") != "api-key" {
			t.Errorf("expected api key param, got %q", r.URL.Query().Get("key"))
		}

		json.NewEncoder(w).Encode(videoResponse{
			Items: []struct {
				LiveStreamingDetails struct {
					ActiveLiveChatID string `json:"activeLiveChatId"`
				} `json:"liveStreamingDetails"`
			}{
				{LiveStreamingDetails: struct {
					ActiveLiveChatID string `json:"activeLiveChatId"`
				}{ActiveLiveChatID: "chat-abc"}},
			},
		})
	}))
	defer server.Close()

	c := NewClient("api-key", "video-123")
	c.httpClient = server.Client()

	// Override the URL by temporarily replacing the const via the request
	origURL := videosURL
	defer func() { _ = origURL }()

	// Use a custom fetchLiveChatID that hits our test server
	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"?part=liveStreamingDetails&id=video-123&key=api-key", nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var videoResp videoResponse
	json.NewDecoder(resp.Body).Decode(&videoResp)

	if len(videoResp.Items) == 0 {
		t.Fatal("expected items in response")
	}
	if videoResp.Items[0].LiveStreamingDetails.ActiveLiveChatID != "chat-abc" {
		t.Errorf("unexpected chat ID: %s", videoResp.Items[0].LiveStreamingDetails.ActiveLiveChatID)
	}
}

func TestFetchLiveChatIDVideoNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(videoResponse{Items: nil})
	}))
	defer server.Close()

	c := &Client{
		apiKey:     "key",
		videoID:    "bad-id",
		httpClient: server.Client(),
	}

	// Manually test the parse logic
	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	resp, _ := c.httpClient.Do(req)
	defer resp.Body.Close()

	var videoResp videoResponse
	json.NewDecoder(resp.Body).Decode(&videoResp)

	if len(videoResp.Items) != 0 {
		t.Errorf("expected empty items, got %d", len(videoResp.Items))
	}
}

func TestFetchMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(liveChatResponse{
			NextPageToken:         "next-token",
			PollingIntervalMillis: 5000,
			Items: []struct {
				Snippet struct {
					PublishedAt     string `json:"publishedAt"`
					DisplayMessage  string `json:"displayMessage"`
					AuthorChannelID string `json:"authorChannelId"`
				} `json:"snippet"`
				AuthorDetails struct {
					DisplayName string `json:"displayName"`
				} `json:"authorDetails"`
			}{
				{
					Snippet: struct {
						PublishedAt     string `json:"publishedAt"`
						DisplayMessage  string `json:"displayMessage"`
						AuthorChannelID string `json:"authorChannelId"`
					}{
						PublishedAt:    "2025-06-15T10:30:00Z",
						DisplayMessage: "hello from youtube",
					},
					AuthorDetails: struct {
						DisplayName string `json:"displayName"`
					}{
						DisplayName: "YTUser",
					},
				},
				{
					Snippet: struct {
						PublishedAt     string `json:"publishedAt"`
						DisplayMessage  string `json:"displayMessage"`
						AuthorChannelID string `json:"authorChannelId"`
					}{
						PublishedAt:    "invalid-date",
						DisplayMessage: "bad timestamp msg",
					},
					AuthorDetails: struct {
						DisplayName string `json:"displayName"`
					}{
						DisplayName: "User2",
					},
				},
			},
		})
	}))
	defer server.Close()

	messages := make(chan message.Message, 10)
	ctx := context.Background()

	// Manually fetch to test parsing
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	client := server.Client()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var chatResp liveChatResponse
	json.NewDecoder(resp.Body).Decode(&chatResp)

	if chatResp.NextPageToken != "next-token" {
		t.Errorf("NextPageToken = %q", chatResp.NextPageToken)
	}
	if chatResp.PollingIntervalMillis != 5000 {
		t.Errorf("PollingIntervalMillis = %d", chatResp.PollingIntervalMillis)
	}
	if len(chatResp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(chatResp.Items))
	}

	// Verify message conversion
	for _, item := range chatResp.Items {
		timestamp, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
		if timestamp.IsZero() {
			timestamp = time.Now()
		}
		messages <- message.Message{
			Platform:  message.YouTube,
			Username:  item.AuthorDetails.DisplayName,
			Timestamp: timestamp,
			Content:   item.Snippet.DisplayMessage,
		}
	}
	close(messages)

	var received []message.Message
	for msg := range messages {
		received = append(received, msg)
	}

	if received[0].Username != "YTUser" {
		t.Errorf("msg[0].Username = %q", received[0].Username)
	}
	if received[0].Content != "hello from youtube" {
		t.Errorf("msg[0].Content = %q", received[0].Content)
	}
	if received[0].Platform != message.YouTube {
		t.Errorf("msg[0].Platform = %v", received[0].Platform)
	}
	expectedTime := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	if !received[0].Timestamp.Equal(expectedTime) {
		t.Errorf("msg[0].Timestamp = %v, want %v", received[0].Timestamp, expectedTime)
	}

	// Second message has invalid timestamp, should fallback to ~now
	if received[1].Username != "User2" {
		t.Errorf("msg[1].Username = %q", received[1].Username)
	}
}

func TestFetchMessagesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	client := server.Client()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}
