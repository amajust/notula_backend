package handlers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type RecordingHandler struct {
	firestore *firestore.Client
}

func NewRecordingHandler(fs *firestore.Client) *RecordingHandler {
	return &RecordingHandler{firestore: fs}
}

// UploadOfflineRecording handles multipart upload of audio files + tags for in-person meetings.
func (h *RecordingHandler) UploadOfflineRecording(c *fiber.Ctx) error {
	uid, ok := c.Locals("uid").(string)
	if !ok || uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	// 1. Parse File
	file, err := c.FormFile("audio")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "audio file is required"})
	}

	// 2. Parse Metadata
	tagsRaw := c.FormValue("tags")
	var tags []string
	if tagsRaw != "" {
		tags = strings.Split(tagsRaw, ",")
		// Trim whitespace from tags
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	title := c.FormValue("title", "Offline Meeting")
	duration := c.FormValue("duration", "0")

	// 3. Save File Locally (for development)
	// In production, this would go to GCS / S3
	uploadDir := "./uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create upload directory"})
		}
	}

	fileID := uuid.New().String()
	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = ".aac" // Default for our Flutter app
	}
	filePath := filepath.Join(uploadDir, fileID+ext)

	if err := c.SaveFile(file, filePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save audio file"})
	}

	// 4. Save Metadata to Firestore
	recordID := uuid.New().String()
	_, err = h.firestore.Collection("recordings").Doc(recordID).Set(context.Background(), map[string]interface{}{
		"id":        recordID,
		"uid":       uid,
		"title":     title,
		"tags":      tags,
		"filePath":  filePath,
		"duration":  duration,
		"createdAt": time.Now(),
		"status":    "recorded",
		"type":      "offline",
	})

	if err != nil {
		// Clean up file if Firestore save fails
		os.Remove(filePath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save metadata to firestore"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":      recordID,
		"message": "In-person meeting recorded and saved",
		"title":   title,
		"tags":    tags,
	})
}
