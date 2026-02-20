package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	content := `
bridge = true

[twitch]
channel = "xqc"

[youtube]
video_id = "dQw4w9WgXcQ"
api_key = "test-api-key"

[hackrtv]
url = "wss://hackr.tv/cable"
channel = "live"
token = "test-token"
alias = "XERAEN"
`
	path := writeTempConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !cfg.Bridge {
		t.Error("expected Bridge to be true")
	}
	if cfg.Twitch.Channel != "xqc" {
		t.Errorf("Twitch.Channel = %q, want %q", cfg.Twitch.Channel, "xqc")
	}
	if cfg.YouTube.VideoID != "dQw4w9WgXcQ" {
		t.Errorf("YouTube.VideoID = %q, want %q", cfg.YouTube.VideoID, "dQw4w9WgXcQ")
	}
	if cfg.YouTube.APIKey != "test-api-key" {
		t.Errorf("YouTube.APIKey = %q, want %q", cfg.YouTube.APIKey, "test-api-key")
	}
	if cfg.HackrTV.URL != "wss://hackr.tv/cable" {
		t.Errorf("HackrTV.URL = %q, want %q", cfg.HackrTV.URL, "wss://hackr.tv/cable")
	}
	if cfg.HackrTV.Channel != "live" {
		t.Errorf("HackrTV.Channel = %q, want %q", cfg.HackrTV.Channel, "live")
	}
	if cfg.HackrTV.Token != "test-token" {
		t.Errorf("HackrTV.Token = %q, want %q", cfg.HackrTV.Token, "test-token")
	}
	if cfg.HackrTV.Alias != "XERAEN" {
		t.Errorf("HackrTV.Alias = %q, want %q", cfg.HackrTV.Alias, "XERAEN")
	}
}

func TestLoadPartial(t *testing.T) {
	content := `
[twitch]
channel = "shroud"
`
	path := writeTempConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Twitch.Channel != "shroud" {
		t.Errorf("Twitch.Channel = %q, want %q", cfg.Twitch.Channel, "shroud")
	}
	if cfg.Bridge {
		t.Error("expected Bridge to be false for partial config")
	}
	if cfg.YouTube.VideoID != "" {
		t.Errorf("YouTube.VideoID = %q, want empty", cfg.YouTube.VideoID)
	}
	if cfg.HackrTV.URL != "" {
		t.Errorf("HackrTV.URL = %q, want empty", cfg.HackrTV.URL)
	}
}

func TestLoadInvalidPath(t *testing.T) {
	_, err := Load("/nonexistent/relay.toml")
	if err == nil {
		t.Fatal("Load() expected error for missing file, got nil")
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := Config{}
	cfg.ApplyDefaults()

	if cfg.HackrTV.Channel != "live" {
		t.Errorf("HackrTV.Channel = %q, want %q", cfg.HackrTV.Channel, "live")
	}
	if cfg.HackrTV.Alias != "relay" {
		t.Errorf("HackrTV.Alias = %q, want %q", cfg.HackrTV.Alias, "relay")
	}
}

func TestApplyDefaultsPreservesExisting(t *testing.T) {
	cfg := Config{
		HackrTV: HackrTVConfig{
			Channel: "custom",
			Alias:   "XERAEN",
		},
	}
	cfg.ApplyDefaults()

	if cfg.HackrTV.Channel != "custom" {
		t.Errorf("HackrTV.Channel = %q, want %q", cfg.HackrTV.Channel, "custom")
	}
	if cfg.HackrTV.Alias != "XERAEN" {
		t.Errorf("HackrTV.Alias = %q, want %q", cfg.HackrTV.Alias, "XERAEN")
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "relay.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}
