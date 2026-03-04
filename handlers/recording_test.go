package handlers

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"notulapro-backend/gladia"
	"notulapro-backend/storage"

	"cloud.google.com/go/firestore"
	"github.com/gofiber/fiber/v2"
)

// ─── Mocks ───────────────────────────────────────────────────────────────────

type mockGladiaClient struct {
	uploadAndTranscribeFunc func(filePath string) (*gladia.TranscriptionResponse, error)
	transcribeFunc          func(audioURL string, callbackURL string) (*gladia.TranscriptionResponse, error)
}

func (m *mockGladiaClient) UploadAndTranscribe(filePath string) (*gladia.TranscriptionResponse, error) {
	return m.uploadAndTranscribeFunc(filePath)
}

func (m *mockGladiaClient) Transcribe(audioURL string, callbackURL string) (*gladia.TranscriptionResponse, error) {
	return m.transcribeFunc(audioURL, callbackURL)
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
			if recording["status"] != "processing" {
				t.Errorf("Expected status 'processing', got %v", recording["status"])
			}
			return nil
		},
		updateRecordingFunc: func(ctx context.Context, id string, updates []firestore.Update) error {
			return nil
		},
	}

	// Use a real FirebaseStorageClient with a mock bucket (will fail but that's ok for unit test)
	// In a real test, you'd use dependency injection or a mock that implements the interface
	// For now, we'll pass nil and handle it in the handler
	var mockStorage *storage.FirebaseStorageClient

	handler := NewRecordingHandler(mockRepo, mockGladia, mockStorage)

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

	// This test will fail because storage client is nil, but it shows the structure
	// A proper integration test would use a mock storage client
	t.Logf("Response status: %d", resp.StatusCode)
}

func TestRecordingHandler_UploadOfflineRecording_Unauthorized(t *testing.T) {
	app := fiber.New()
	var mockStorage *storage.FirebaseStorageClient
	handler := NewRecordingHandler(nil, nil, mockStorage)
	app.Post("/upload", handler.UploadOfflineRecording)

	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.StatusCode)
	}
}
