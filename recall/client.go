package recall

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"notulapro-backend/recall/events"
)

// Client is a thin wrapper around the Recall.ai REST API.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// New creates a Recall.ai client using env vars.
func New() *Client {
	region := os.Getenv("RECALL_REGION")
	if region == "" {
		region = "us-west-2"
	}
	return &Client{
		apiKey:  os.Getenv("RECALL_API_KEY"),
		baseURL: fmt.Sprintf("https://%s.recall.ai/api/v1", region),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// ─── Request / Response types ────────────────────────────────────────────────

type CreateBotRequest struct {
	MeetingURL           string                 `json:"meeting_url"`
	BotName              string                 `json:"bot_name"`
	JoinAt               *time.Time             `json:"join_at,omitempty"`
	RecordingConfig      map[string]interface{} `json:"recording_config,omitempty"`
	TranscriptionOptions map[string]interface{} `json:"transcription_options,omitempty"`
}

// BotResponse is now in recall/events/types.go
// Transcription types are now in recall/events/types.go

type CreateTranscriptRequest struct {
	Provider    map[string]interface{} `json:"provider"`
	Diarization map[string]interface{} `json:"diarization,omitempty"`
}

type TranscriptResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

// TranscriptElement is now in recall/events/types.go

// ─── Bot methods ─────────────────────────────────────────────────────────────

// CreateBot sends or schedules Notbot for a meeting.
func (c *Client) CreateBot(meetingURL string, botName string, joinAt *time.Time) (*events.BotResponse, error) {
	if botName == "" {
		botName = "Notbot"
	}

	// Set up real-time transcription with Gladia
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		log.Printf("[WARNING] BASE_URL env var is missing, Webhook URL will be invalid!")
	}
	realtimeURL := baseURL + "/api/v1/webhook/recall/realtime"

	recordingConfig := map[string]interface{}{
		"transcript": map[string]interface{}{
			"provider": map[string]interface{}{
				"gladia_v2_streaming": map[string]interface{}{},
			},
		},
		"realtime_endpoints": []map[string]interface{}{
			{
				"type": "webhook",
				"url":  realtimeURL,
				"events": []string{
					"transcript.data",
				},
			},
		},
	}

	req := CreateBotRequest{
		MeetingURL:      meetingURL,
		BotName:         botName,
		JoinAt:          joinAt,
		RecordingConfig: recordingConfig,
	}
	return c.postBot(req)
}

// GetBot fetches the current status of a bot.
func (c *Client) GetBot(botID string) (*events.BotResponse, error) {
	url := fmt.Sprintf("%s/bot/%s/", c.baseURL, botID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.checkStatus(resp); err != nil {
		return nil, err
	}

	var bot events.BotResponse
	if err := json.NewDecoder(resp.Body).Decode(&bot); err != nil {
		return nil, err
	}

	// Check if any status change indicates a fatal error
	for _, sc := range bot.StatusChanges {
		if code, ok := sc["code"].(string); ok && code == "fatal" {
			errMsg, _ := sc["message"].(string)
			subCode, _ := sc["sub_code"].(string)
			return &bot, fmt.Errorf("bot fatal error: %s (sub_code: %s)", errMsg, subCode)
		}
	}

	return &bot, nil
}

// LeaveBot tells the bot to leave the call immediately.
func (c *Client) LeaveBot(botID string) error {
	url := fmt.Sprintf("%s/bot/%s/leave_call/", c.baseURL, botID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.checkStatus(resp)
}

// SendChatMessage sends a message to the meeting chat via the bot.
func (c *Client) SendChatMessage(botID string, text string) error {
	url := fmt.Sprintf("%s/bot/%s/send_chat_message/", c.baseURL, botID)

	payload := map[string]string{
		"message": text,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.checkStatus(resp)
}

// ─── Transcription methods ────────────────────────────────────────────────────

// StartAsyncTranscription kicks off a Gladia async transcription job.
func (c *Client) StartAsyncTranscription(recordingID string) error {
	body := CreateTranscriptRequest{
		Provider: map[string]interface{}{
			"gladia": map[string]interface{}{},
		},
		Diarization: map[string]interface{}{
			"use_separate_streams_when_available": true,
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/recording/%s/create_transcript/", c.baseURL, recordingID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.checkStatus(resp)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (c *Client) postBot(payload CreateBotRequest) (*events.BotResponse, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/bot/", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.checkStatus(resp); err != nil {
		return nil, err
	}

	var bot events.BotResponse
	if err := json.NewDecoder(resp.Body).Decode(&bot); err != nil {
		return nil, err
	}
	return &bot, nil
}

// GetTranscript fetches the completed transcript for a recording.
func (c *Client) GetTranscript(recordingID string) ([]events.TranscriptElement, error) {
	url := fmt.Sprintf("%s/recording/%s/transcript/", c.baseURL, recordingID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.checkStatus(resp); err != nil {
		return nil, err
	}

	var transcript []events.TranscriptElement
	if err := json.NewDecoder(resp.Body).Decode(&transcript); err != nil {
		return nil, err
	}
	return transcript, nil
}

// DeleteMedia deletes the recording media for a bot to stop storage charges.
func (c *Client) DeleteMedia(botID string) error {
	url := fmt.Sprintf("%s/bot/%s/delete_media/", c.baseURL, botID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.checkStatus(resp)
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Token "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

func (c *Client) checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("recall.ai error %d: %s", resp.StatusCode, string(body))
}
