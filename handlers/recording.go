package handlers

import (
	"context"
	"fmt"
	"log"
	"notulapro-backend/gladia"
	"strings"
	"time"

	"notulapro-backend/storage"

	"cloud.google.com/go/firestore"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// GladiaClient defines the interface for interacting with Gladia.io.
type GladiaClient interface {
	Transcribe(audioURL string, callbackURL string) (*gladia.TranscriptionResponse, error)
	UploadAndTranscribe(filePath string) (*gladia.TranscriptionResponse, error)
}

// RecordingRepository defines the interface for persisting recording data.
type RecordingRepository interface {
	SaveRecording(ctx context.Context, recording map[string]interface{}) error
	UpdateRecording(ctx context.Context, id string, updates []firestore.Update) error
}

type RecordingHandler struct {
	repo          RecordingRepository
	gladia        GladiaClient
	storageClient *storage.FirebaseStorageClient
}

func NewRecordingHandler(repo RecordingRepository, g GladiaClient, storageClient *storage.FirebaseStorageClient) *RecordingHandler {
	return &RecordingHandler{repo: repo, gladia: g, storageClient: storageClient}
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

	// 3. Generate recording ID
	recordID := uuid.New().String()
	storagePath := fmt.Sprintf("recordings/%s/%s.aac", uid, recordID)

	// 4. Upload to Firebase Storage
	if h.storageClient != nil {
		err = h.storageClient.UploadFile(file, storagePath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "failed to upload to Firebase Storage",
				"details": err.Error(),
			})
		}
	} else {
		// Fallback for testing - just log
		log.Printf("[FirebaseStorage] Storage client not available, skipping upload (test mode)")
	}

	// 5. Save Metadata to Firestore with status "processing"
	err = h.repo.SaveRecording(context.Background(), map[string]interface{}{
		"id":        recordID,
		"uid":       uid,
		"title":     title,
		"tags":      tags,
		"duration":  duration,
		"mediaPath": storagePath,
		"createdAt": time.Now(),
		"status":    "processing",
		"type":      "offline",
	})

	if err != nil {
		// Clean up file if Firestore save fails
		log.Printf("ERROR: Failed to save recording to Firestore: %v", err)
		if h.storageClient != nil {
			h.storageClient.DeleteFile(storagePath)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "failed to save metadata to firestore",
			"details": err.Error(),
		})
	}

	// 6. Trigger Gladia Transcription Asynchronously
	// We do this after saving to Firestore so we have a record even if Gladia fails
	if h.storageClient != nil {
		// Generate signed URL for Gladia to access the file
		signedURL, err := h.storageClient.GenerateSignedURL(storagePath, 60) // 60 minutes expiry
		if err != nil {
			log.Printf("failed to generate signed URL for %s: %v", recordID, err)
		} else {
			// Trigger transcription with callback URL
			callbackURL := fmt.Sprintf("%s/api/v1/webhook/gladia?recording_id=%s",
				c.BaseURL(), recordID)

			gladiaRes, err := h.gladia.Transcribe(signedURL, callbackURL)
			if err != nil {
				// Log error but don't fail the request - we can retry later if needed
				log.Printf("failed to trigger gladia transcription for %s: %v", recordID, err)
			} else {
				// Update Firestore with Gladia ID
				_ = h.repo.UpdateRecording(context.Background(), recordID, []firestore.Update{
					{Path: "gladiaId", Value: gladiaRes.ID},
				})
				log.Printf("Gladia transcription started for %s (gladia_id: %s)", recordID, gladiaRes.ID)
			}
		}
	} else {
		log.Printf("[Gladia] Storage client not available, skipping transcription (test mode)")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":      recordID,
		"message": "In-person meeting recorded and saved. Transcription started.",
		"title":   title,
		"tags":    tags,
	})
}
