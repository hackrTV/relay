package display

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"relay/internal/message"
)

type Printer struct {
	twitchColor   *color.Color
	youtubeColor  *color.Color
	hackrtvColor  *color.Color
	usernameColor *color.Color
	dimColor      *color.Color
}

func NewPrinter() *Printer {
	return &Printer{
		twitchColor:   color.New(color.FgMagenta, color.Bold),
		youtubeColor:  color.New(color.FgRed, color.Bold),
		hackrtvColor:  color.New(color.FgGreen, color.Bold),
		usernameColor: color.New(color.FgCyan),
		dimColor:      color.New(color.FgHiBlack),
	}
}

func (p *Printer) Print(msg message.Message) {
	// Line 1: [TW] username • HH:MM:SS
	// Line 2:     message content (indented)
	// Line 3: thin separator
	var platformStr string
	switch msg.Platform {
	case message.Twitch:
		platformStr = p.twitchColor.Sprint("[TTV]")
	case message.YouTube:
		platformStr = p.youtubeColor.Sprint("[YT_]")
	case message.HackrTV:
		platformStr = p.hackrtvColor.Sprint("[HTV]")
	}

	timestamp := p.dimColor.Sprint(msg.Timestamp.Local().Format("15:04:05"))

	// Line 1: header
	fmt.Fprintf(os.Stdout, "%s %s %s %s\n",
		platformStr,
		p.usernameColor.Sprint(msg.Username),
		p.dimColor.Sprint("•"),
		timestamp,
	)
	// Line 2: indented message
	fmt.Fprintf(os.Stdout, "    %s\n", msg.Content)
	// Line 3: thin separator
	fmt.Fprintln(os.Stdout, p.dimColor.Sprint("────────────────────────────────"))
}

func (p *Printer) Run(messages <-chan message.Message) {
	for msg := range messages {
		p.Print(msg)
	}
}
