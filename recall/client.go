package recall

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
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
	TranscriptionOptions map[string]interface{} `json:"transcription_options,omitempty"`
}

type BotResponse struct {
	ID            string                   `json:"id"`
	StatusChanges []map[string]interface{} `json:"status_changes"`
	Recordings    []struct {
		ID string `json:"id"`
	} `json:"recordings"`
}

type CreateTranscriptRequest struct {
	Provider    map[string]interface{} `json:"provider"`
	Diarization map[string]interface{} `json:"diarization,omitempty"`
}

// ─── Bot methods ─────────────────────────────────────────────────────────────

// CreateBot sends or schedules Notbot for a meeting.
func (c *Client) CreateBot(meetingURL string, joinAt *time.Time) (*BotResponse, error) {
	req := CreateBotRequest{
		MeetingURL: meetingURL,
		BotName:    "Notbot",
		JoinAt:     joinAt,
		TranscriptionOptions: map[string]interface{}{
			"provider": "gladia",
		},
	}
	return c.postBot(req)
}

// GetBot fetches the current status of a bot.
func (c *Client) GetBot(botID string) (*BotResponse, error) {
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

	var bot BotResponse
	if err := json.NewDecoder(resp.Body).Decode(&bot); err != nil {
		return nil, err
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

func (c *Client) postBot(payload CreateBotRequest) (*BotResponse, error) {
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

	var bot BotResponse
	if err := json.NewDecoder(resp.Body).Decode(&bot); err != nil {
		return nil, err
	}
	return &bot, nil
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
