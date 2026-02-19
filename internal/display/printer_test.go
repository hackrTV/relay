package display

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"relay/internal/message"
)

func init() {
	// Disable color output for deterministic test assertions
	color.NoColor = true
}

func capturePrint(p *Printer, msg message.Message) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p.Print(msg)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestPrintTwitch(t *testing.T) {
	p := NewPrinter()
	msg := message.Message{
		Platform:  message.Twitch,
		Username:  "testuser",
		Timestamp: time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		Content:   "hello chat",
	}

	output := capturePrint(p, msg)

	if !strings.Contains(output, "[TTV]") {
		t.Errorf("expected [TTV] tag, got: %s", output)
	}
	if !strings.Contains(output, "testuser") {
		t.Errorf("expected username, got: %s", output)
	}
	if !strings.Contains(output, "hello chat") {
		t.Errorf("expected message content, got: %s", output)
	}
}

func TestPrintYouTube(t *testing.T) {
	p := NewPrinter()
	msg := message.Message{
		Platform:  message.YouTube,
		Username:  "ytuser",
		Timestamp: time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		Content:   "yt message",
	}

	output := capturePrint(p, msg)

	if !strings.Contains(output, "[YT_]") {
		t.Errorf("expected [YT_] tag, got: %s", output)
	}
}

func TestPrintHackrTV(t *testing.T) {
	p := NewPrinter()
	msg := message.Message{
		Platform:  message.HackrTV,
		Username:  "xeraen",
		Timestamp: time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		Content:   "hackrtv message",
	}

	output := capturePrint(p, msg)

	if !strings.Contains(output, "[HTV]") {
		t.Errorf("expected [HTV] tag, got: %s", output)
	}
	if !strings.Contains(output, "xeraen") {
		t.Errorf("expected username, got: %s", output)
	}
}

func TestPrintOutputFormat(t *testing.T) {
	p := NewPrinter()
	msg := message.Message{
		Platform:  message.Twitch,
		Username:  "someone",
		Timestamp: time.Date(2025, 6, 1, 9, 5, 3, 0, time.UTC),
		Content:   "test content",
	}

	output := capturePrint(p, msg)
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), output)
	}

	// Line 1: header with platform, username, bullet, timestamp
	if !strings.Contains(lines[0], "someone") {
		t.Errorf("line 1 missing username: %q", lines[0])
	}

	// Line 2: indented content
	if !strings.HasPrefix(lines[1], "    ") {
		t.Errorf("line 2 should be indented: %q", lines[1])
	}
	if !strings.Contains(lines[1], "test content") {
		t.Errorf("line 2 missing content: %q", lines[1])
	}

	// Line 3: separator
	if !strings.Contains(lines[2], "────") {
		t.Errorf("line 3 should be separator: %q", lines[2])
	}
}

func TestRun(t *testing.T) {
	p := NewPrinter()
	ch := make(chan message.Message, 2)

	ch <- message.Message{
		Platform:  message.Twitch,
		Username:  "a",
		Timestamp: time.Now(),
		Content:   "msg1",
	}
	ch <- message.Message{
		Platform:  message.YouTube,
		Username:  "b",
		Timestamp: time.Now(),
		Content:   "msg2",
	}
	close(ch)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	p.Run(ch)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "msg1") || !strings.Contains(output, "msg2") {
		t.Errorf("Run should print all messages, got: %s", output)
	}
}
