package twitch

import (
	"relay/internal/message"
	"testing"
)

func TestParsePrivMsg(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantOk   bool
		wantUser string
		wantText string
	}{
		{
			name:     "standard PRIVMSG",
			line:     ":cooluser!cooluser@cooluser.tmi.twitch.tv PRIVMSG #channel :hello world",
			wantOk:   true,
			wantUser: "cooluser",
			wantText: "hello world",
		},
		{
			name:     "message with colons in content",
			line:     ":user!user@user.tmi.twitch.tv PRIVMSG #ch :time is 12:34:56",
			wantOk:   true,
			wantUser: "user",
			wantText: "time is 12:34:56",
		},
		{
			name:   "PING message",
			line:   "PING :tmi.twitch.tv",
			wantOk: false,
		},
		{
			name:   "JOIN message",
			line:   ":user!user@user.tmi.twitch.tv JOIN #channel",
			wantOk: false,
		},
		{
			name:   "empty line",
			line:   "",
			wantOk: false,
		},
		{
			name:   "PRIVMSG without colon prefix",
			line:   "user PRIVMSG #channel :hello",
			wantOk: false,
		},
		{
			name:   "malformed prefix no exclamation",
			line:   ":useronly PRIVMSG #channel :hello",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, ok := parsePrivMsg(tt.line)
			if ok != tt.wantOk {
				t.Fatalf("parsePrivMsg() ok = %v, want %v", ok, tt.wantOk)
			}
			if !ok {
				return
			}
			if msg.Username != tt.wantUser {
				t.Errorf("Username = %q, want %q", msg.Username, tt.wantUser)
			}
			if msg.Content != tt.wantText {
				t.Errorf("Content = %q, want %q", msg.Content, tt.wantText)
			}
			if msg.Platform != message.Twitch {
				t.Errorf("Platform = %v, want Twitch", msg.Platform)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("UPPERCASE")
	if c.channel != "uppercase" {
		t.Errorf("NewClient did not lowercase channel: got %q", c.channel)
	}
}
