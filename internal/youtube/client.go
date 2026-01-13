package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"relay/internal/message"
)

const (
	liveChatMessagesURL = "https://www.googleapis.com/youtube/v3/liveChat/messages"
	videosURL           = "https://www.googleapis.com/youtube/v3/videos"
)

type Client struct {
	apiKey      string
	videoID     string
	liveChatID  string
	httpClient  *http.Client
	pageToken   string
	pollingRate time.Duration
}

func NewClient(apiKey, videoID string) *Client {
	return &Client{
		apiKey:      apiKey,
		videoID:     videoID,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		pollingRate: 3 * time.Second,
	}
}

// liveChatResponse represents the YouTube Live Chat API response
type liveChatResponse struct {
	NextPageToken         string `json:"nextPageToken"`
	PollingIntervalMillis int    `json:"pollingIntervalMillis"`
	Items                 []struct {
		Snippet struct {
			PublishedAt     string `json:"publishedAt"`
			DisplayMessage  string `json:"displayMessage"`
			AuthorChannelID string `json:"authorChannelId"`
		} `json:"snippet"`
		AuthorDetails struct {
			DisplayName string `json:"displayName"`
		} `json:"authorDetails"`
	} `json:"items"`
}

// videoResponse represents the YouTube Videos API response
type videoResponse struct {
	Items []struct {
		LiveStreamingDetails struct {
			ActiveLiveChatID string `json:"activeLiveChatId"`
		} `json:"liveStreamingDetails"`
	} `json:"items"`
}

func (c *Client) Connect(ctx context.Context, messages chan<- message.Message) error {
	// First, get the live chat ID from the video
	if err := c.fetchLiveChatID(ctx); err != nil {
		return fmt.Errorf("failed to get live chat ID: %w", err)
	}

	// Poll for messages
	ticker := time.NewTicker(c.pollingRate)
	defer ticker.Stop()

	// Initial fetch
	if err := c.fetchMessages(ctx, messages); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.fetchMessages(ctx, messages); err != nil {
				// Log error but continue polling
				fmt.Printf("YouTube fetch error: %v\n", err)
			}
		}
	}
}

func (c *Client) fetchLiveChatID(ctx context.Context) error {
	params := url.Values{}
	params.Set("part", "liveStreamingDetails")
	params.Set("id", c.videoID)
	params.Set("key", c.apiKey)

	reqURL := fmt.Sprintf("%s?%s", videosURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var videoResp videoResponse
	if err := json.NewDecoder(resp.Body).Decode(&videoResp); err != nil {
		return err
	}

	if len(videoResp.Items) == 0 {
		return fmt.Errorf("video not found: %s", c.videoID)
	}

	c.liveChatID = videoResp.Items[0].LiveStreamingDetails.ActiveLiveChatID
	if c.liveChatID == "" {
		return fmt.Errorf("video %s does not have an active live chat", c.videoID)
	}

	return nil
}

func (c *Client) fetchMessages(ctx context.Context, messages chan<- message.Message) error {
	params := url.Values{}
	params.Set("part", "snippet,authorDetails")
	params.Set("liveChatId", c.liveChatID)
	params.Set("key", c.apiKey)
	if c.pageToken != "" {
		params.Set("pageToken", c.pageToken)
	}

	reqURL := fmt.Sprintf("%s?%s", liveChatMessagesURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var chatResp liveChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return err
	}

	// Update page token for next request
	c.pageToken = chatResp.NextPageToken

	// Update polling rate if provided
	if chatResp.PollingIntervalMillis > 0 {
		c.pollingRate = time.Duration(chatResp.PollingIntervalMillis) * time.Millisecond
	}

	// Send messages
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

	return nil
}
