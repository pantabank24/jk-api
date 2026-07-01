package linenotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type textMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type pushRequest struct {
	To       string        `json:"to"`
	Messages []textMessage `json:"messages"`
}

type replyRequest struct {
	ReplyToken string        `json:"replyToken"`
	Messages   []textMessage `json:"messages"`
}

// SendText sends a plain-text push message to a LINE user or group chat.
// Silently skips when LINE_CHANNEL_ACCESS_TOKEN is not set.
func SendText(targetID, text string) error {
	token := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if token == "" || targetID == "" {
		return nil
	}
	payload := pushRequest{
		To:       targetID,
		Messages: []textMessage{{Type: "text", Text: text}},
	}
	return doPost(token, "https://api.line.me/v2/bot/message/push", payload)
}

// ReplyText sends a plain-text reply using a webhook replyToken (single-use, 30s TTL).
func ReplyText(replyToken, text string) error {
	token := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if token == "" || replyToken == "" {
		return nil
	}
	payload := replyRequest{
		ReplyToken: replyToken,
		Messages:   []textMessage{{Type: "text", Text: text}},
	}
	return doPost(token, "https://api.line.me/v2/bot/message/reply", payload)
}

func doPost(token, url string, payload interface{}) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("LINE API returned %d", resp.StatusCode)
	}
	return nil
}

// WebhookPayload is the top-level structure LINE POSTs to the webhook URL.
type WebhookPayload struct {
	Destination string         `json:"destination"`
	Events      []WebhookEvent `json:"events"`
}

type WebhookEvent struct {
	Type       string      `json:"type"`
	ReplyToken string      `json:"replyToken"`
	Source     EventSource `json:"source"`
}

type EventSource struct {
	Type    string `json:"type"`
	UserID  string `json:"userId"`
	GroupID string `json:"groupId"`
	RoomID  string `json:"roomId"`
}
