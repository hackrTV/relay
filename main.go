package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"relay/internal/display"
	"relay/internal/hackrtv"
	"relay/internal/message"
	"relay/internal/twitch"
	"relay/internal/uplink"
	"relay/internal/youtube"
)

func main() {
	// CLI flags
	twitchChannel := flag.String("twitch-channel", "", "Twitch channel name to watch")
	youtubeVideoID := flag.String("youtube-video-id", "", "YouTube video ID for live stream")
	youtubeAPIKey := flag.String("youtube-api-key", "", "YouTube Data API key (or set YOUTUBE_API_KEY env)")
	hackrtvURL := flag.String("hackrtv-url", "", "hackr.tv ActionCable WebSocket URL (e.g. wss://hackr.tv/cable)")
	hackrtvChannel := flag.String("hackrtv-channel", "live", "hackr.tv chat channel slug")
	hackrtvToken := flag.String("hackrtv-token", "", "hackr.tv admin API token (or set HACKRTV_API_TOKEN env)")
	hackrtvAlias := flag.String("hackrtv-alias", "relay", "hackr.tv hackr alias for auth")
	bridge := flag.Bool("bridge", false, "Bridge Twitch/YouTube chat to hackr.tv via Uplink API")
	flag.Parse()

	// Check for env fallbacks
	if *youtubeAPIKey == "" {
		*youtubeAPIKey = os.Getenv("YOUTUBE_API_KEY")
	}
	if *hackrtvToken == "" {
		*hackrtvToken = os.Getenv("HACKRTV_API_TOKEN")
	}

	// Validate inputs
	if *twitchChannel == "" && *youtubeVideoID == "" && *hackrtvURL == "" {
		fmt.Fprintln(os.Stderr, "Error: At least one platform is required (--twitch-channel, --youtube-video-id, or --hackrtv-url)")
		flag.Usage()
		os.Exit(1)
	}

	if *youtubeVideoID != "" && *youtubeAPIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: --youtube-api-key (or YOUTUBE_API_KEY env) is required for YouTube")
		os.Exit(1)
	}

	if *bridge && (*hackrtvURL == "" || *hackrtvToken == "") {
		fmt.Fprintln(os.Stderr, "Error: --bridge requires --hackrtv-url and --hackrtv-token")
		os.Exit(1)
	}

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nShutting down...")
		cancel()
	}()

	// Create unified message channel
	messages := make(chan message.Message, 100)

	// Fan-out: printer always receives; uplink receives non-HTV when bridging
	printerCh := make(chan message.Message, 100)
	var uplinkCh chan message.Message

	if *bridge {
		uplinkCh = make(chan message.Message, 100)
	}

	go func() {
		for msg := range messages {
			// In bridge mode, suppress HTV echoes of our own bridged messages
			if uplinkCh != nil && isBridgeEcho(msg, *hackrtvAlias) {
				continue
			}
			printerCh <- msg
			if uplinkCh != nil && msg.Platform != message.HackrTV {
				select {
				case uplinkCh <- msg:
				default:
					// drop if uplink can't keep up — don't block printer
				}
			}
		}
		close(printerCh)
		if uplinkCh != nil {
			close(uplinkCh)
		}
	}()

	// Start printer goroutine
	printer := display.NewPrinter()
	go printer.Run(printerCh)

	// Start uplink bridge if enabled
	if *bridge {
		uplinkClient, err := uplink.NewClient(*hackrtvURL, *hackrtvToken, *hackrtvAlias, *hackrtvChannel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Uplink client error: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "Bridge mode enabled — forwarding Twitch/YouTube chat to hackr.tv")
		go uplinkClient.Run(ctx, uplinkCh)
	}

	// Track active connections
	var wg sync.WaitGroup

	// Start Twitch client if configured
	if *twitchChannel != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := twitch.NewClient(*twitchChannel)
			fmt.Fprintf(os.Stderr, "Connecting to Twitch channel: %s\n", *twitchChannel)
			if err := client.Connect(ctx, messages); err != nil && ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "Twitch error: %v\n", err)
			}
		}()
	}

	// Start YouTube client if configured
	if *youtubeVideoID != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := youtube.NewClient(*youtubeAPIKey, *youtubeVideoID)
			fmt.Fprintf(os.Stderr, "Connecting to YouTube video: %s\n", *youtubeVideoID)
			if err := client.Connect(ctx, messages); err != nil && ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "YouTube error: %v\n", err)
			}
		}()
	}

	// Start hackr.tv client if configured
	if *hackrtvURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := hackrtv.NewClient(*hackrtvURL, *hackrtvToken, *hackrtvAlias, *hackrtvChannel)
			fmt.Fprintf(os.Stderr, "Connecting to hackr.tv channel: %s\n", *hackrtvChannel)
			if err := client.Connect(ctx, messages); err != nil && ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "hackr.tv error: %v\n", err)
			}
		}()
	}

	// Wait for all clients to finish
	wg.Wait()
	close(messages)
}

// isBridgeEcho returns true if an HTV message is an echo of a bridged
// Twitch/YouTube message sent by our own relay alias.
func isBridgeEcho(msg message.Message, relayAlias string) bool {
	return msg.Platform == message.HackrTV &&
		strings.EqualFold(msg.Username, relayAlias) &&
		(strings.HasPrefix(msg.Content, "[TTV] ") || strings.HasPrefix(msg.Content, "[YT_] "))
}
