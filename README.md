# Relay

A CLI tool for viewing Twitch, YouTube Live, and hackr.tv chat streams in a unified, color-coded display.

## Features

- Real-time chat messages from Twitch, YouTube Live, and hackr.tv in a single view
- Color-coded platform identifiers (purple for Twitch, red for YouTube, green for hackr.tv)
- Highlighted usernames for readability
- Timestamps in local time
- No Twitch credentials required (anonymous read-only access)
- hackr.tv streams via ActionCable WebSocket with per-hackr token auth
- Bridge mode: forward Twitch/YouTube messages into hackr.tv live chat via the Admin Uplink API

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

# Watch hackr.tv chat only (requires HACKRTV_API_TOKEN env var)
relay --hackrtv-url=wss://hackr.tv/cable

# Watch all three simultaneously
relay --twitch-channel=channelname \
      --youtube-video-id=VIDEO_ID --youtube-api-key=YOUR_API_KEY \
      --hackrtv-url=wss://hackr.tv/cable

# Bridge Twitch chat into hackr.tv (messages appear in the grid)
relay --bridge \
      --twitch-channel=channelname \
      --hackrtv-url=wss://hackr.tv/cable \
      --hackrtv-token=YOUR_TOKEN \
      --hackrtv-alias=XERAEN
```

### Config File

Instead of passing many flags, you can use a TOML config file:

```bash
cp relay.example.toml relay.toml
# Edit relay.toml with your values
relay --config relay.toml
```

CLI flags override config file values, and env vars fill in anything not set by either:

**CLI flag > env var > config file > default**

```bash
# Config file provides base settings, flag overrides one value
relay --config relay.toml --twitch-channel=other
```

See `relay.example.toml` for all available fields.

### Environment Variables

| Variable | Flag fallback | Description |
|---|---|---|
| `YOUTUBE_API_KEY` | `--youtube-api-key` | YouTube Data API key |
| `HACKRTV_API_TOKEN` | `--hackrtv-token` | hackr.tv API token (per-hackr) |

### hackr.tv Flags

| Flag | Default | Description |
|---|---|---|
| `--hackrtv-url` | *(required)* | ActionCable WebSocket URL |
| `--hackrtv-channel` | `live` | Chat channel slug |
| `--hackrtv-token` | `HACKRTV_API_TOKEN` env | API token (per-hackr) |
| `--hackrtv-alias` | `relay` | hackr alias for auth |
| `--bridge` | `false` | Forward Twitch/YouTube chat to hackr.tv via Uplink API |

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
└─────────────┘  │    ┌──────────────┐     ┌───────────┐
                 ├───►│   messages   │──┬─►│ Printer   │
┌─────────────┐  │    │   channel    │  │  │ goroutine │
│ YouTube API │──┤    └──────────────┘  │  └───────────┘
│  goroutine  │  │                      │
└─────────────┘  │                      │  ┌───────────┐
                 │       (--bridge)     └─►│ Uplink    │
┌─────────────┐  │       TTV/YT only       │ goroutine │
│ hackr.tv WS │──┘                         └───────────┘
│  goroutine  │
└─────────────┘
```

- **Twitch Client**: Connects to Twitch IRC anonymously using the `justinfan` convention. Parses PRIVMSG lines and handles PING/PONG keepalive.

- **YouTube Client**: Polls the YouTube Data API v3 liveChatMessages endpoint. Tracks page tokens to avoid duplicate messages and respects the API's suggested polling interval.

- **hackr.tv Client**: Connects to hackr.tv via ActionCable WebSocket. Authenticates with an admin token, subscribes to a LiveChatChannel, receives initial packet history and live packets in real-time. Filters dropped (moderated) packets.

- **Uplink Client** (`--bridge`): POSTs Twitch/YouTube messages to hackr.tv's Admin Uplink API as `[TTV] user: message` or `[YT_] user: message`. hackr.tv messages are excluded to prevent echo loops, and echoed bridge messages from the relay alias are suppressed in the local display. Backs off on 429 rate limits.

- **Printer**: Reads from the unified message channel and outputs color-coded, formatted messages to stdout.

## Project Structure

```
relay/
├── main.go                        # Entry point, CLI flags, orchestration
├── relay.example.toml             # Example config file
├── internal/
│   ├── config/config.go           # TOML config loading and defaults
│   ├── message/message.go         # Unified message struct and platform enum
│   ├── twitch/client.go           # Twitch IRC client
│   ├── youtube/client.go          # YouTube Live Chat API client
│   ├── hackrtv/client.go          # hackr.tv ActionCable WebSocket client
│   ├── uplink/client.go           # hackr.tv Admin Uplink API client (bridge mode)
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
