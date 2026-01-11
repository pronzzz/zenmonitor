package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pronzzz/zenmonitor/internal/config"
)

type Sender interface {
	Send(message string) error
}

type Service struct {
	Senders []Sender
}

func NewService(cfg []config.NotificationConfig) *Service {
	var senders []Sender
	for _, n := range cfg {
		switch n.Type {
		case "telegram":
			if n.Token != "" && n.ChatID != "" {
				senders = append(senders, &TelegramSender{Token: n.Token, ChatID: n.ChatID})
			}
		case "slack":
			if n.WebhookURL != "" {
				senders = append(senders, &SlackSender{WebhookURL: n.WebhookURL})
			}
		}
	}
	return &Service{Senders: senders}
}

func (s *Service) Notify(monitorName string, isUp bool, wasUp bool) {
	status := "DOWN"
	if isUp {
		status = "UP"
	}
	
	emoji := "🔴"
	if isUp {
		emoji = "🟢"
	}

	msg := fmt.Sprintf("%s Monitor *%s* is %s at %s", emoji, monitorName, status, time.Now().Format(time.RFC1123))

	for _, sender := range s.Senders {
		go func(snd Sender) {
			// Ignore errors for now or log them
			_ = snd.Send(msg)
		}(sender)
	}
}

// --- Telegram ---

type TelegramSender struct {
	Token  string
	ChatID string
}

func (t *TelegramSender) Send(message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.Token)
	payload := map[string]string{
		"chat_id":    t.ChatID,
		"text":       message,
		"parse_mode": "Markdown", // used *bold*
	}
	return postJSON(url, payload)
}

// --- Slack ---

type SlackSender struct {
	WebhookURL string
}

func (s *SlackSender) Send(message string) error {
	payload := map[string]string{
		"text": message,
	}
	return postJSON(s.WebhookURL, payload)
}

// --- Helper ---

func postJSON(url string, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("api request failed with status: %d", resp.StatusCode)
	}
	return nil
}
