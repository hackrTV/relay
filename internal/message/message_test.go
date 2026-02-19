package message

import "testing"

func TestPlatformString(t *testing.T) {
	tests := []struct {
		platform Platform
		want     string
	}{
		{Twitch, "TTV"},
		{YouTube, "YT_"},
		{HackrTV, "HTV"},
		{Platform(99), "???"},
	}

	for _, tt := range tests {
		got := tt.platform.String()
		if got != tt.want {
			t.Errorf("Platform(%d).String() = %q, want %q", tt.platform, got, tt.want)
		}
	}
}
