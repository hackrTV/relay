package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"relay/internal/display"
	"relay/internal/message"
	"relay/internal/twitch"
	"relay/internal/youtube"
)

func main() {
	// CLI flags
	twitchChannel := flag.String("twitch-channel", "", "Twitch channel name to watch")
	youtubeVideoID := flag.String("youtube-video-id", "", "YouTube video ID for live stream")
	youtubeAPIKey := flag.String("youtube-api-key", "", "YouTube Data API key (or set YOUTUBE_API_KEY env)")
	flag.Parse()

	// Check for YouTube API key in environment if not provided via flag
	if *youtubeAPIKey == "" {
		*youtubeAPIKey = os.Getenv("YOUTUBE_API_KEY")
	}

	// Validate inputs
	if *twitchChannel == "" && *youtubeVideoID == "" {
		fmt.Fprintln(os.Stderr, "Error: At least one of --twitch-channel or --youtube-video-id is required")
		flag.Usage()
		os.Exit(1)
	}

	if *youtubeVideoID != "" && *youtubeAPIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: --youtube-api-key (or YOUTUBE_API_KEY env) is required for YouTube")
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

	// Start printer goroutine
	printer := display.NewPrinter()
	go printer.Run(messages)

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

	// Wait for all clients to finish
	wg.Wait()
	close(messages)
}
