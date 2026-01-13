# Relay

A CLI tool for viewing Twitch and YouTube Live chat streams in a unified, color-coded display.

## Features

- Real-time chat messages from Twitch and YouTube Live in a single view
- Color-coded platform identifiers (purple for Twitch, red for YouTube)
- Highlighted usernames for readability
- Timestamps in local time
- No Twitch credentials required (anonymous read-only access)

## Installation

```bash
git clone https://github.com/hackrTV/relay.git
cd relay
go build -o relay .
```

## Usage

```bash
# Watch Twitch chat only (no credentials needed)
relay --twitch-channel=channelname

# Watch YouTube Live chat only
relay --youtube-video-id=VIDEO_ID --youtube-api-key=YOUR_API_KEY

# Watch both simultaneously
relay --twitch-channel=channelname --youtube-video-id=VIDEO_ID --youtube-api-key=YOUR_API_KEY
```

The YouTube API key can also be set via the `YOUTUBE_API_KEY` environment variable.

## Output Format

```
[TW] username • 14:32:05
    Hello everyone!
────────────────────────────────
[YT] username • 14:32:07
    What's up chat
────────────────────────────────
```

## YouTube API Setup

1. Go to the Google Cloud Console (https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the "YouTube Data API v3"
4. Navigate to Credentials and create an API key
5. Use the key with `--youtube-api-key` or set `YOUTUBE_API_KEY`

## Design

Relay uses a concurrent architecture with goroutines:

```
┌─────────────┐
│ Twitch IRC  │──┐
│  goroutine  │  │    ┌──────────────┐     ┌──────────┐
└─────────────┘  ├───►│   messages   │────►│ Printer  │
                 │    │   channel    │     │goroutine │
┌─────────────┐  │    └──────────────┘     └──────────┘
│ YouTube API │──┘
│  goroutine  │
└─────────────┘
```

- **Twitch Client**: Connects to Twitch IRC anonymously using the `justinfan` convention. Parses PRIVMSG lines and handles PING/PONG keepalive.

- **YouTube Client**: Polls the YouTube Data API v3 liveChatMessages endpoint. Tracks page tokens to avoid duplicate messages and respects the API's suggested polling interval.

- **Printer**: Reads from the unified message channel and outputs color-coded, formatted messages to stdout.

## Project Structure

```
relay/
├── main.go                     # Entry point, CLI flags, orchestration
├── internal/
│   ├── message/message.go      # Unified message struct and platform enum
│   ├── twitch/client.go        # Twitch IRC client
│   ├── youtube/client.go       # YouTube Live Chat API client
│   └── display/printer.go      # Color-coded terminal output
├── go.mod
└── go.sum
```

## License

This project is released into the public domain under the Unlicense. See UNLICENSE for details.
