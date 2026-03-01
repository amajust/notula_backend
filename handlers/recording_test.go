package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRecordingHandler_UploadOfflineRecording_Unauthorized(t *testing.T) {
	app := fiber.New()

	// Create handler with nil firestore since we shouldn't reach it
	handler := NewRecordingHandler(nil)

	// Route that explicitly DOES NOT set the "uid" local variable
	app.Post("/upload", handler.UploadOfflineRecording)

	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	resp, err := app.Test(req)

	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}

	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", fiber.StatusUnauthorized, resp.StatusCode)
	}
}

func TestRecordingHandler_UploadOfflineRecording_MissingAudio(t *testing.T) {
	app := fiber.New()
	handler := NewRecordingHandler(nil)

	// Middleware to inject a mock "uid", bypassing the unauthorized check
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("uid", "test-user-123")
		return c.Next()
	})

	app.Post("/upload", handler.UploadOfflineRecording)

	// Create a multipart request but WITHOUT an "audio" file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add some other field just to make it a valid multipart
	_ = writer.WriteField("title", "Test Title")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)

	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", fiber.StatusBadRequest, resp.StatusCode)
	}
}
