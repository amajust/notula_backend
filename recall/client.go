package recall

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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

	req := CreateBotRequest{
		MeetingURL: meetingURL,
		BotName:    botName,
		JoinAt:     joinAt,
		RecordingConfig: map[string]interface{}{
			"meeting_metadata": map[string]interface{}{},
		},
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

// DeleteBot deletes the bot entirely from Recall.ai.
func (c *Client) DeleteBot(botID string) error {
	url := fmt.Sprintf("%s/bot/%s/", c.baseURL, botID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
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
			"gladia_v2_async": map[string]interface{}{
				"diarization": true,
			},
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

// GetTranscript fetches the completed transcript using the modern 2-step flow.
func (c *Client) GetTranscript(transcriptID string) ([]events.TranscriptElement, error) {
	// Step 1: Fetch transcript metadata to get the download URL
	metaURL := fmt.Sprintf("%s/transcript/%s/", c.baseURL, transcriptID)
	log.Printf("[RecallClient] GetTranscript: Fetching metadata for ID %s at %s", transcriptID, metaURL)
	req, err := http.NewRequest(http.MethodGet, metaURL, nil)
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
		return nil, fmt.Errorf("failed to fetch transcript metadata: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read transcript metadata body: %w", err)
	}
	log.Printf("[RecallClient] Transcript metadata response: %s", string(body))

	var meta struct {
		Data struct {
			DownloadURL string `json:"download_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("failed to decode transcript metadata: %w", err)
	}

	downloadURL := meta.Data.DownloadURL
	if downloadURL == "" {
		return nil, fmt.Errorf("no download URL found in transcript metadata")
	}

	// Step 2: Download the actual JSON transcript from the download URL
	log.Printf("[RecallClient] Downloading transcript from %s", downloadURL)
	transResp, err := http.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download transcript from URL: %w", err)
	}
	defer transResp.Body.Close()

	var rawSegments []json.RawMessage
	bodyBytes, err := io.ReadAll(transResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read transcript body: %w", err)
	}
	limit := 500
	if len(bodyBytes) < limit {
		limit = len(bodyBytes)
	}
	log.Printf("[RecallClient] Raw Transcript JSON (first %d chars): %s", limit, string(bodyBytes[:limit]))

	if err := json.Unmarshal(bodyBytes, &rawSegments); err != nil {
		return nil, fmt.Errorf("failed to decode transcript JSON: %w", err)
	}

	type recallWord struct {
		Text           string `json:"text"`
		StartTimestamp struct {
			Relative float64 `json:"relative"`
		} `json:"start_timestamp"`
		EndTimestamp struct {
			Relative float64 `json:"relative"`
		} `json:"end_timestamp"`
	}

	type recallSegment struct {
		Participant struct {
			ID   interface{} `json:"id"`
			Name string      `json:"name"`
		} `json:"participant"`
		Words []recallWord `json:"words"`
	}

	var segments []recallSegment
	if err := json.Unmarshal(bodyBytes, &segments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal into recallSegments: %w", err)
	}

	var transcript []events.TranscriptElement
	for _, s := range segments {
		if len(s.Words) == 0 {
			continue
		}

		var textBuilder strings.Builder
		for i, w := range s.Words {
			if i > 0 {
				textBuilder.WriteString(" ")
			}
			textBuilder.WriteString(w.Text)
		}

		speaker := s.Participant.Name
		if speaker == "" {
			if s.Participant.ID != nil {
				speaker = fmt.Sprintf("Speaker %v", s.Participant.ID)
			} else {
				speaker = "Unknown"
			}
		}

		start := s.Words[0].StartTimestamp.Relative
		minutes := int(start) / 60
		seconds := int(start) % 60
		timestamp := fmt.Sprintf("%02d:%02d", minutes, seconds)

		transcript = append(transcript, events.TranscriptElement{
			Speaker:   speaker,
			Text:      textBuilder.String(),
			Start:     start,
			End:       s.Words[len(s.Words)-1].EndTimestamp.Relative,
			Timestamp: timestamp,
		})
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

// ─── Calendar methods ─────────────────────────────────────────────────────────

// CreateCalendar creates a new calendar object in Recall.
func (c *Client) CreateCalendar() (*events.Calendar, error) {
	url := c.baseURL + "/calendar/"
	req, err := http.NewRequest(http.MethodPost, url, nil)
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

	var cal events.Calendar
	if err := json.NewDecoder(resp.Body).Decode(&cal); err != nil {
		return nil, err
	}
	return &cal, nil
}

// GetCalendarOauthURL fetches the OAuth connect URL for a specific calendar.
func (c *Client) GetCalendarOauthURL(calendarID string) (string, error) {
	url := fmt.Sprintf("%s/calendar/%s/oauth_url/", c.baseURL, calendarID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := c.checkStatus(resp); err != nil {
		return "", err
	}

	var res events.CalendarOauthURL
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.OauthURL, nil
}

// GetCalendar fetches a single calendar's status.
func (c *Client) GetCalendar(calendarID string) (*events.Calendar, error) {
	url := fmt.Sprintf("%s/calendar/%s/", c.baseURL, calendarID)
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

	var cal events.Calendar
	if err := json.NewDecoder(resp.Body).Decode(&cal); err != nil {
		return nil, err
	}
	return &cal, nil
}

// UpdateCalendar updates calendar settings (e.g. automatic_recording).
func (c *Client) UpdateCalendar(calendarID string, payload map[string]interface{}) (*events.Calendar, error) {
	url := fmt.Sprintf("%s/calendar/%s/", c.baseURL, calendarID)

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(data))
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

	var cal events.Calendar
	if err := json.NewDecoder(resp.Body).Decode(&cal); err != nil {
		return nil, err
	}
	return &cal, nil
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
	return fmt.Errorf("recall_status:%d body:%s", resp.StatusCode, string(body))
}
