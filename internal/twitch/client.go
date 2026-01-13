package twitch

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"relay/internal/message"
)

const (
	ircServer = "irc.chat.twitch.tv:6667"
)

type Client struct {
	channel string
	conn    net.Conn
}

func NewClient(channel string) *Client {
	return &Client{
		channel: strings.ToLower(channel),
	}
}

func (c *Client) Connect(ctx context.Context, messages chan<- message.Message) error {
	var err error
	c.conn, err = net.Dial("tcp", ircServer)
	if err != nil {
		return fmt.Errorf("failed to connect to Twitch IRC: %w", err)
	}
	defer c.conn.Close()

	// Generate anonymous username
	username := fmt.Sprintf("justinfan%d", rand.Intn(99999)+1)

	// Send IRC registration
	fmt.Fprintf(c.conn, "NICK %s\r\n", username)
	fmt.Fprintf(c.conn, "JOIN #%s\r\n", c.channel)

	reader := bufio.NewReader(c.conn)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			line, err := reader.ReadString('\n')
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return fmt.Errorf("read error: %w", err)
			}

			line = strings.TrimSpace(line)

			// Respond to PING to stay connected
			if strings.HasPrefix(line, "PING") {
				fmt.Fprintf(c.conn, "PONG%s\r\n", strings.TrimPrefix(line, "PING"))
				continue
			}

			// Parse PRIVMSG
			msg, ok := parsePrivMsg(line)
			if ok {
				messages <- msg
			}
		}
	}
}

// parsePrivMsg parses IRC PRIVMSG format:
// :username!username@username.tmi.twitch.tv PRIVMSG #channel :message content
func parsePrivMsg(line string) (message.Message, bool) {
	if !strings.Contains(line, "PRIVMSG") {
		return message.Message{}, false
	}

	// Extract username from prefix
	if !strings.HasPrefix(line, ":") {
		return message.Message{}, false
	}

	parts := strings.SplitN(line[1:], "!", 2)
	if len(parts) < 2 {
		return message.Message{}, false
	}
	username := parts[0]

	// Find message content after " :"
	msgIdx := strings.Index(line, " :")
	if msgIdx == -1 {
		return message.Message{}, false
	}

	// Skip the PRIVMSG prefix part, find the actual message
	afterPrivmsg := strings.SplitN(line, "PRIVMSG", 2)
	if len(afterPrivmsg) < 2 {
		return message.Message{}, false
	}

	contentIdx := strings.Index(afterPrivmsg[1], ":")
	if contentIdx == -1 {
		return message.Message{}, false
	}
	content := afterPrivmsg[1][contentIdx+1:]

	return message.Message{
		Platform:  message.Twitch,
		Username:  username,
		Timestamp: time.Now(),
		Content:   content,
	}, true
}
