# Relay

A CLI tool for viewing Twitch, YouTube Live, and hackr.tv chat streams in a unified, color-coded display.

## Features

- Real-time chat messages from Twitch, YouTube Live, and hackr.tv in a single view
- Color-coded platform identifiers (purple for Twitch, red for YouTube, green for hackr.tv)
- Highlighted usernames for readability
- Timestamps in local time
- No Twitch credentials required (anonymous read-only access)
- hackr.tv streams via ActionCable WebSocket with admin token auth

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

# Watch hackr.tv chat only (requires HACKR_ADMIN_API_TOKEN env var)
relay --hackrtv-url=wss://hackr.tv/cable

# Watch all three simultaneously
relay --twitch-channel=channelname \
      --youtube-video-id=VIDEO_ID --youtube-api-key=YOUR_API_KEY \
      --hackrtv-url=wss://hackr.tv/cable
```

### Environment Variables

| Variable | Flag fallback | Description |
|---|---|---|
| `YOUTUBE_API_KEY` | `--youtube-api-key` | YouTube Data API key |
| `HACKR_ADMIN_API_TOKEN` | `--hackrtv-token` | hackr.tv admin API token |

### hackr.tv Flags

| Flag | Default | Description |
|---|---|---|
| `--hackrtv-url` | *(required)* | ActionCable WebSocket URL |
| `--hackrtv-channel` | `live` | Chat channel slug |
| `--hackrtv-token` | `HACKR_ADMIN_API_TOKEN` env | Admin API token |
| `--hackrtv-alias` | `relay` | hackr alias for auth |

## Output Format

```
[TTV] username • 14:32:05
    Hello everyone!
────────────────────────────────
[YT_] username • 14:32:07
    What's up chat
────────────────────────────────
[HTV] xeraen • 14:32:09
    Welcome to the grid
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
│  goroutine  │  │
└─────────────┘  │    ┌──────────────┐     ┌──────────┐
                 ├───►│   messages   │────►│ Printer  │
┌─────────────┐  │    │   channel    │     │goroutine │
│ YouTube API │──┤    └──────────────┘     └──────────┘
│  goroutine  │  │
└─────────────┘  │
                 │
┌─────────────┐  │
│ hackr.tv WS │──┘
│  goroutine  │
└─────────────┘
```

- **Twitch Client**: Connects to Twitch IRC anonymously using the `justinfan` convention. Parses PRIVMSG lines and handles PING/PONG keepalive.

- **YouTube Client**: Polls the YouTube Data API v3 liveChatMessages endpoint. Tracks page tokens to avoid duplicate messages and respects the API's suggested polling interval.

- **hackr.tv Client**: Connects to hackr.tv via ActionCable WebSocket. Authenticates with an admin token, subscribes to a LiveChatChannel, receives initial packet history and live packets in real-time. Filters dropped (moderated) packets.

- **Printer**: Reads from the unified message channel and outputs color-coded, formatted messages to stdout.

## Project Structure

```
relay/
├── main.go                        # Entry point, CLI flags, orchestration
├── internal/
│   ├── message/message.go         # Unified message struct and platform enum
│   ├── twitch/client.go           # Twitch IRC client
│   ├── youtube/client.go          # YouTube Live Chat API client
│   ├── hackrtv/client.go          # hackr.tv ActionCable WebSocket client
│   └── display/printer.go         # Color-coded terminal output
├── go.mod
└── go.sum
```

## Testing

```bash
go test ./...
```

## License

This project is released into the public domain under the Unlicense. See UNLICENSE for details.
