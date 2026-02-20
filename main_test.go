package main

import (
	"testing"

	"relay/internal/message"
)

func TestIsBridgeEcho(t *testing.T) {
	tests := []struct {
		name       string
		msg        message.Message
		relayAlias string
		want       bool
	}{
		{
			name:       "HTV echo of TTV message from relay alias",
			msg:        message.Message{Platform: message.HackrTV, Username: "XERAEN", Content: "[TTV] nightbot: !commands"},
			relayAlias: "XERAEN",
			want:       true,
		},
		{
			name:       "HTV echo of YT message from relay alias",
			msg:        message.Message{Platform: message.HackrTV, Username: "relay", Content: "[YT_] viewer: hello"},
			relayAlias: "relay",
			want:       true,
		},
		{
			name:       "case-insensitive alias match",
			msg:        message.Message{Platform: message.HackrTV, Username: "xeraen", Content: "[TTV] user: hi"},
			relayAlias: "XERAEN",
			want:       true,
		},
		{
			name:       "different alias — not an echo",
			msg:        message.Message{Platform: message.HackrTV, Username: "someone_else", Content: "[TTV] user: hi"},
			relayAlias: "XERAEN",
			want:       false,
		},
		{
			name:       "HTV message without bridge prefix — not an echo",
			msg:        message.Message{Platform: message.HackrTV, Username: "XERAEN", Content: "hello grid"},
			relayAlias: "XERAEN",
			want:       false,
		},
		{
			name:       "TTV message — not an echo (wrong platform)",
			msg:        message.Message{Platform: message.Twitch, Username: "XERAEN", Content: "[TTV] user: hi"},
			relayAlias: "XERAEN",
			want:       false,
		},
		{
			name:       "HTV prefix without space — not an echo",
			msg:        message.Message{Platform: message.HackrTV, Username: "XERAEN", Content: "[TTV]no space"},
			relayAlias: "XERAEN",
			want:       false,
		},
		{
			name:       "user typing fake bridge format — not suppressed (different alias)",
			msg:        message.Message{Platform: message.HackrTV, Username: "troll", Content: "[TTV] fake: lol"},
			relayAlias: "XERAEN",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBridgeEcho(tt.msg, tt.relayAlias)
			if got != tt.want {
				t.Errorf("isBridgeEcho() = %v, want %v", got, tt.want)
			}
		})
	}
}
