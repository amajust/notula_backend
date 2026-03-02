package gladia

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

const BaseURL = "https://api.gladia.io/v2"

type Client struct {
	APIKey     string
	HTTPClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
	}
}

type UploadResponse struct {
	AudioURL string `json:"audio_url"`
}

type TranscriptionResponse struct {
	ID        string `json:"id"`
	ResultURL string `json:"result_url"`
}

// Upload uploads a local file to Gladia and returns a temporary audio_url.
func (c *Client) Upload(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("audio", file.Name())
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	req, err := http.NewRequest("POST", BaseURL+"/upload", body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-gladia-key", c.APIKey)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		resBody, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("gladia upload error (status %d): %s", res.StatusCode, string(resBody))
	}

	var upRes UploadResponse
	if err := json.NewDecoder(res.Body).Decode(&upRes); err != nil {
		return "", fmt.Errorf("failed to decode upload response: %w", err)
	}

	return upRes.AudioURL, nil
}

// Transcribe triggers transcription for a given audio_url.
func (c *Client) Transcribe(audioURL string) (*TranscriptionResponse, error) {
	payload := map[string]string{
		"audio_url": audioURL,
	}
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", BaseURL+"/pre-recorded", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create transcription request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-gladia-key", c.APIKey)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send transcription request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
		resBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("gladia transcription error (status %d): %s", res.StatusCode, string(resBody))
	}

	var transRes TranscriptionResponse
	if err := json.NewDecoder(res.Body).Decode(&transRes); err != nil {
		return nil, fmt.Errorf("failed to decode transcription response: %w", err)
	}

	return &transRes, nil
}

// UploadAndTranscribe performs the two-step process: Upload then Transcribe.
func (c *Client) UploadAndTranscribe(filePath string) (*TranscriptionResponse, error) {
	audioURL, err := c.Upload(filePath)
	if err != nil {
		return nil, err
	}
	return c.Transcribe(audioURL)
}

type ResultResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"` // "queued", "processing", "done", "error"
	Result any    `json:"result"`
}

func (c *Client) GetStatus(transcriptionID string) (*ResultResponse, error) {
	req, err := http.NewRequest("GET", BaseURL+"/pre-recorded/"+transcriptionID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-gladia-key", c.APIKey)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var result ResultResponse
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
