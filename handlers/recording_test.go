package handlers

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"notulapro-backend/gladia"

	"cloud.google.com/go/firestore"
	"github.com/gofiber/fiber/v2"
)

// ─── Mocks ───────────────────────────────────────────────────────────────────

type mockGladiaClient struct {
	uploadAndTranscribeFunc func(filePath string) (*gladia.TranscriptionResponse, error)
}

func (m *mockGladiaClient) UploadAndTranscribe(filePath string) (*gladia.TranscriptionResponse, error) {
	return m.uploadAndTranscribeFunc(filePath)
}

type mockRecordingRepository struct {
	saveRecordingFunc   func(ctx context.Context, recording map[string]interface{}) error
	updateRecordingFunc func(ctx context.Context, id string, updates []firestore.Update) error
}

func (m *mockRecordingRepository) SaveRecording(ctx context.Context, recording map[string]interface{}) error {
	return m.saveRecordingFunc(ctx, recording)
}
func (m *mockRecordingRepository) UpdateRecording(ctx context.Context, id string, updates []firestore.Update) error {
	return m.updateRecordingFunc(ctx, id, updates)
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestRecordingHandler_UploadOfflineRecording_Success(t *testing.T) {
	app := fiber.New()

	mockGladia := &mockGladiaClient{
		uploadAndTranscribeFunc: func(filePath string) (*gladia.TranscriptionResponse, error) {
			return &gladia.TranscriptionResponse{ID: "gladia-123"}, nil
		},
	}

	mockRepo := &mockRecordingRepository{
		saveRecordingFunc: func(ctx context.Context, recording map[string]interface{}) error {
			if recording["uid"] != "user-123" {
				t.Errorf("Expected UID user-123, got %v", recording["uid"])
			}
			return nil
		},
		updateRecordingFunc: func(ctx context.Context, id string, updates []firestore.Update) error {
			return nil
		},
	}

	handler := NewRecordingHandler(mockRepo, mockGladia)

	app.Post("/upload", func(c *fiber.Ctx) error {
		c.Locals("uid", "user-123")
		return handler.UploadOfflineRecording(c)
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("audio", "test.wav")
	part.Write([]byte("fake audio content"))
	_ = writer.WriteField("title", "Test Meeting")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, _ := app.Test(req)

	if resp.StatusCode != fiber.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_UploadOfflineRecording_Unauthorized(t *testing.T) {
	app := fiber.New()
	handler := NewRecordingHandler(nil, nil)
	app.Post("/upload", handler.UploadOfflineRecording)

	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.StatusCode)
	}
}
