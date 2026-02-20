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

	"relay/internal/config"
	"relay/internal/display"
	"relay/internal/hackrtv"
	"relay/internal/message"
	"relay/internal/twitch"
	"relay/internal/uplink"
	"relay/internal/youtube"
)

func main() {
	// CLI flags
	configPath := flag.String("config", "", "Path to TOML config file")
	twitchChannel := flag.String("twitch-channel", "", "Twitch channel name to watch")
	youtubeVideoID := flag.String("youtube-video-id", "", "YouTube video ID for live stream")
	youtubeAPIKey := flag.String("youtube-api-key", "", "YouTube Data API key (or set YOUTUBE_API_KEY env)")
	hackrtvURL := flag.String("hackrtv-url", "", "hackr.tv ActionCable WebSocket URL (e.g. wss://hackr.tv/cable)")
	hackrtvChannel := flag.String("hackrtv-channel", "", "hackr.tv chat channel slug")
	hackrtvToken := flag.String("hackrtv-token", "", "hackr.tv admin API token (or set HACKRTV_API_TOKEN env)")
	hackrtvAlias := flag.String("hackrtv-alias", "", "hackr.tv hackr alias for auth")
	bridge := flag.Bool("bridge", false, "Bridge Twitch/YouTube chat to hackr.tv via Uplink API")
	flag.Parse()

	// Load config file if specified
	var cfg config.Config
	if *configPath != "" {
		var err error
		cfg, err = config.Load(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Apply defaults for fields that have them
	cfg.ApplyDefaults()

	// Override config with explicitly-set CLI flags
	flagsSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		flagsSet[f.Name] = true
	})

	if flagsSet["twitch-channel"] {
		cfg.Twitch.Channel = *twitchChannel
	}
	if flagsSet["youtube-video-id"] {
		cfg.YouTube.VideoID = *youtubeVideoID
	}
	if flagsSet["youtube-api-key"] {
		cfg.YouTube.APIKey = *youtubeAPIKey
	}
	if flagsSet["hackrtv-url"] {
		cfg.HackrTV.URL = *hackrtvURL
	}
	if flagsSet["hackrtv-channel"] {
		cfg.HackrTV.Channel = *hackrtvChannel
	}
	if flagsSet["hackrtv-token"] {
		cfg.HackrTV.Token = *hackrtvToken
	}
	if flagsSet["hackrtv-alias"] {
		cfg.HackrTV.Alias = *hackrtvAlias
	}
	if flagsSet["bridge"] {
		cfg.Bridge = *bridge
	}

	// Env var fallbacks for fields still empty
	if cfg.YouTube.APIKey == "" {
		cfg.YouTube.APIKey = os.Getenv("YOUTUBE_API_KEY")
	}
	if cfg.HackrTV.Token == "" {
		cfg.HackrTV.Token = os.Getenv("HACKRTV_API_TOKEN")
	}

	// Validate inputs
	if cfg.Twitch.Channel == "" && cfg.YouTube.VideoID == "" && cfg.HackrTV.URL == "" {
		fmt.Fprintln(os.Stderr, "Error: At least one platform is required (--twitch-channel, --youtube-video-id, or --hackrtv-url)")
		flag.Usage()
		os.Exit(1)
	}

	if cfg.YouTube.VideoID != "" && cfg.YouTube.APIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: --youtube-api-key (or YOUTUBE_API_KEY env) is required for YouTube")
		os.Exit(1)
	}

	if cfg.Bridge && (cfg.HackrTV.URL == "" || cfg.HackrTV.Token == "") {
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

	if cfg.Bridge {
		uplinkCh = make(chan message.Message, 100)
	}

	go func() {
		for msg := range messages {
			// In bridge mode, suppress HTV echoes of our own bridged messages
			if uplinkCh != nil && isBridgeEcho(msg, cfg.HackrTV.Alias) {
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
	if cfg.Bridge {
		uplinkClient, err := uplink.NewClient(cfg.HackrTV.URL, cfg.HackrTV.Token, cfg.HackrTV.Alias, cfg.HackrTV.Channel)
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
	if cfg.Twitch.Channel != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := twitch.NewClient(cfg.Twitch.Channel)
			fmt.Fprintf(os.Stderr, "Connecting to Twitch channel: %s\n", cfg.Twitch.Channel)
			if err := client.Connect(ctx, messages); err != nil && ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "Twitch error: %v\n", err)
			}
		}()
	}

	// Start YouTube client if configured
	if cfg.YouTube.VideoID != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := youtube.NewClient(cfg.YouTube.APIKey, cfg.YouTube.VideoID)
			fmt.Fprintf(os.Stderr, "Connecting to YouTube video: %s\n", cfg.YouTube.VideoID)
			if err := client.Connect(ctx, messages); err != nil && ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "YouTube error: %v\n", err)
			}
		}()
	}

	// Start hackr.tv client if configured
	if cfg.HackrTV.URL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := hackrtv.NewClient(cfg.HackrTV.URL, cfg.HackrTV.Token, cfg.HackrTV.Alias, cfg.HackrTV.Channel)
			fmt.Fprintf(os.Stderr, "Connecting to hackr.tv channel: %s\n", cfg.HackrTV.Channel)
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
