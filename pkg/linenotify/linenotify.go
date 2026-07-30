package linenotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type message map[string]interface{}

type pushRequest struct {
	To       string    `json:"to"`
	Messages []message `json:"messages"`
}

type replyRequest struct {
	ReplyToken string    `json:"replyToken"`
	Messages   []message `json:"messages"`
}

func textMessage(text string) message {
	return message{"type": "text", "text": text}
}

// SendText sends a plain-text push message to a LINE user or group chat.
// Silently skips when LINE_CHANNEL_ACCESS_TOKEN is not set.
func SendText(targetID, text string) error {
	token := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if token == "" || targetID == "" {
		return nil
	}
	payload := pushRequest{To: targetID, Messages: []message{textMessage(text)}}
	return doPost(token, "https://api.line.me/v2/bot/message/push", payload, nil)
}

// SendFlex pushes a Flex Message. contents is the bubble/carousel object; altText
// is what shows in the chat list and on clients that can't render Flex.
// Silently skips when LINE_CHANNEL_ACCESS_TOKEN is not set.
func SendFlex(targetID, altText string, contents interface{}) error {
	token := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if token == "" || targetID == "" {
		return nil
	}
	payload := pushRequest{To: targetID, Messages: []message{{
		"type":     "flex",
		"altText":  altText,
		"contents": contents,
	}}}
	return doPost(token, "https://api.line.me/v2/bot/message/push", payload, nil)
}

// ReplyText sends a plain-text reply using a webhook replyToken (single-use, 30s TTL).
func ReplyText(replyToken, text string) error {
	token := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if token == "" || replyToken == "" {
		return nil
	}
	payload := replyRequest{ReplyToken: replyToken, Messages: []message{textMessage(text)}}
	return doPost(token, "https://api.line.me/v2/bot/message/reply", payload, nil)
}

// QuotaStatus reports how much of the channel's monthly push-message allowance
// has been used. Feeds the usage dashboard on the LINE settings page.
type QuotaStatus struct {
	// Type is "limited" (a monthly cap applies) or "none" (unlimited plan).
	Type string `json:"type"`
	// Limit is the monthly cap; 0 when Type is "none".
	Limit int `json:"limit"`
	// Used counts messages sent this month that COUNT AGAINST the cap
	// (replies and other free-tier messages are excluded by LINE).
	Used int `json:"used"`
	// Remaining is Limit - Used, floored at 0; 0 and meaningless when unlimited.
	Remaining int `json:"remaining"`
}

// GetQuota reads the channel's monthly limit and this month's consumption.
// Returns nil when no access token is configured.
func GetQuota() (*QuotaStatus, error) {
	token := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if token == "" {
		return nil, nil
	}

	var quota struct {
		Type  string `json:"type"`
		Value int    `json:"value"`
	}
	if err := doGet(token, "https://api.line.me/v2/bot/message/quota", &quota); err != nil {
		return nil, err
	}

	var consumption struct {
		TotalUsage int `json:"totalUsage"`
	}
	if err := doGet(token, "https://api.line.me/v2/bot/message/quota/consumption", &consumption); err != nil {
		return nil, err
	}

	status := &QuotaStatus{Type: quota.Type, Limit: quota.Value, Used: consumption.TotalUsage}
	if status.Type == "limited" && status.Limit > status.Used {
		status.Remaining = status.Limit - status.Used
	}
	return status, nil
}

func doGet(token, url string, out interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("LINE API returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func doPost(token, url string, payload interface{}, out interface{}) error {
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
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
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
