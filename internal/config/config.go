package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Bridge  bool          `toml:"bridge"`
	Twitch  TwitchConfig  `toml:"twitch"`
	YouTube YouTubeConfig `toml:"youtube"`
	HackrTV HackrTVConfig `toml:"hackrtv"`
}

type TwitchConfig struct {
	Channel string `toml:"channel"`
}

type YouTubeConfig struct {
	VideoID string `toml:"video_id"`
	APIKey  string `toml:"api_key"`
}

type HackrTVConfig struct {
	URL     string `toml:"url"`
	Channel string `toml:"channel"`
	Token   string `toml:"token"`
	Alias   string `toml:"alias"`
}

// Load reads and decodes a TOML config file from the given path.
func Load(path string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("reading config file: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config file: %w", err)
	}

	return cfg, nil
}

// ApplyDefaults sets default values for fields that have them.
func (c *Config) ApplyDefaults() {
	if c.HackrTV.Channel == "" {
		c.HackrTV.Channel = "live"
	}
	if c.HackrTV.Alias == "" {
		c.HackrTV.Alias = "relay"
	}
}
