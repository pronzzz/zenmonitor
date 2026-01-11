package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the root of monitors.yaml
type Config struct {
	Global        GlobalConfig         `yaml:"global"`
	Notifications []NotificationConfig `yaml:"notifications"`
	Monitors      []MonitorConfig      `yaml:"monitors"`
}

type GlobalConfig struct {
	CheckInterval string `yaml:"check_interval"`
	HistoryDays   int    `yaml:"history_days"`
}

type NotificationConfig struct {
	Type       string `yaml:"type"`
	Token      string `yaml:"token,omitempty"`
	ChatID     string `yaml:"chat_id,omitempty"`
	WebhookURL string `yaml:"webhook_url,omitempty"`
	
	// Internal parsed fields?
}

type MonitorConfig struct {
	Name         string `yaml:"name"`
	Type         string `yaml:"type"` // http, tcp, icmp
	URL          string `yaml:"url,omitempty"`
	Host         string `yaml:"host,omitempty"`
	Port         int    `yaml:"port,omitempty"`
	Method       string `yaml:"method,omitempty"` // GET, POST
	ExpectStatus int    `yaml:"expect_status,omitempty"`
	Interval     string `yaml:"interval,omitempty"` // Override global
}

// LoadConfig reads and parses the YAML config
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	// Set defaults before unmarshaling?
	// Zero values might be tricky, but let's parse first
	
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}

	// Validate / Set Defaults
	if cfg.Global.CheckInterval == "" {
		cfg.Global.CheckInterval = "60s"
	}
	if cfg.Global.HistoryDays == 0 {
		cfg.Global.HistoryDays = 90
	}

	for i := range cfg.Monitors {
		m := &cfg.Monitors[i]
		if m.Type == "" {
			// Infer type
			if m.URL != "" {
				m.Type = "http"
			} else if m.Host != "" && m.Port != 0 {
				m.Type = "tcp"
			} else if m.Host != "" {
				m.Type = "icmp"
			}
		}
		if m.Method == "" {
			m.Method = "GET"
		}
		if m.ExpectStatus == 0 {
			m.ExpectStatus = 200
		}
	}

	return &cfg, nil
}

// Helper to parse duration string
func ParseDuration(d string) time.Duration {
	dur, err := time.ParseDuration(d)
	if err != nil {
		return 60 * time.Second
	}
	return dur
}
