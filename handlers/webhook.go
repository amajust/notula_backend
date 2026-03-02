package handlers

import (
	"context"
	"fmt"
	"log"
	"os"

	"notulapro-backend/storage"

	"github.com/gofiber/fiber/v2"
)

// WebhookHandler handles incoming notifications.
type WebhookHandler struct {
	recall  RecallClient
	botRepo BotRepository
	recRepo RecordingRepository
	storage *storage.GCSClient
	gladia  GladiaClient
}

func NewWebhookHandler(r RecallClient, br BotRepository, rr RecordingRepository, s *storage.GCSClient, g GladiaClient) *WebhookHandler {
	return &WebhookHandler{recall: r, botRepo: br, recRepo: rr, storage: s, gladia: g}
}

// RecallWebhook handles incoming notifications from Recall.ai.
func (h *WebhookHandler) RecallWebhook(c *fiber.Ctx) error {
	var payload struct {
		Event string `json:"event"`
		Data  struct {
			BotID  string `json:"bot_id"`
			Status struct {
				Code    string `json:"code"`
				SubCode string `json:"sub_code"`
			} `json:"status"`
			RecordingID string `json:"recording_id"`
		} `json:"data"`
	}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	log.Printf("Received Recall webhook: %s for bot %s", payload.Event, payload.Data.BotID)

	switch payload.Event {
	case "bot.status_change":
		if payload.Data.Status.Code == "done" || payload.Data.Status.SubCode == "recording_done" {
			h.handleBotDone(c, payload.Data.BotID)
		}
	case "transcript.done":
		// We no longer handle Recall's internal transcript.done directly here if we use Gladia.
		// However, keeping it for backward compatibility or dual-use if needed.
		h.handleTranscriptDone(c, payload.Data.BotID)
	case "gladia.transcription.done": // Custom event if we handle Gladia results here
		// Actually, Gladia's webhook hits its own endpoint.
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *WebhookHandler) handleBotDone(c *fiber.Ctx, botID string) {
	log.Printf("Bot %s finished recording, triggering transcription...", botID)

	// 1. Fetch bot details to get recording ID
	bot, err := h.recall.GetBot(botID)
	if err != nil {
		log.Printf("Failed to get bot %s: %v", botID, err)
		return
	}

	if len(bot.Recordings) == 0 {
		log.Printf("No recordings found for bot %s", botID)
		return
	}

	recordingURL := bot.Recordings[0].MediaShortcuts.VideoMixed.URL

	// 2. Trigger asynchronous transcription via Gladia
	callbackURL := fmt.Sprintf("%s/api/v1/webhook/gladia?bot_id=%s", os.Getenv("BASE_URL"), botID)
	_, err = h.gladia.Transcribe(recordingURL, callbackURL)
	if err != nil {
		log.Printf("Failed to start Gladia transcription for bot %s: %v", botID, err)
	}

	// 3. Update status in repository
	if err := h.botRepo.UpdateBotStatus(context.Background(), botID, "recorded"); err != nil {
		log.Printf("Failed to update bot %s status: %v", botID, err)
	}
}

func (h *WebhookHandler) handleTranscriptDone(c *fiber.Ctx, botID string) {
	log.Printf("Transcript for bot %s is ready, fetching...", botID)

	transcript, err := h.recall.GetTranscript(botID)
	if err != nil {
		log.Printf("Failed to fetch transcript for bot %s: %v", botID, err)
		return
	}

	// 3. Persist transcript to Firestore via repository
	if err := h.botRepo.SaveTranscript(context.Background(), botID, transcript); err != nil {
		log.Printf("Failed to save transcript for bot %s: %v", botID, err)
	}

	// 4. Archive recording to GCS in background
	go h.archiveToGCS(botID)

	log.Printf("Successfully processed transcript for bot %s", botID)
}

// GladiaWebhook handles incoming results from Gladia.
func (h *WebhookHandler) GladiaWebhook(c *fiber.Ctx) error {
	botID := c.Query("bot_id")
	if botID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bot_id is required"})
	}

	var payload struct {
		Event string `json:"event"`
		Data  struct {
			ID            string `json:"id"`
			Status        string `json:"status"`
			Transcription any    `json:"transcription"`
		} `json:"data"`
	}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	if payload.Event != "transcription.done" {
		log.Printf("Gladia webhook event %s for bot %s - ignoring", payload.Event, botID)
		return c.SendStatus(fiber.StatusOK)
	}

	log.Printf("Gladia transcript for bot %s is ready, saving...", botID)

	// Persist transcript to Firestore
	if err := h.botRepo.SaveTranscript(c.Context(), botID, payload.Data.Transcription); err != nil {
		log.Printf("Failed to save Gladia transcript for bot %s: %v", botID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save transcript"})
	}

	// Archive to GCS in background
	go h.archiveToGCS(botID)

	return c.SendStatus(fiber.StatusOK)
}

func (h *WebhookHandler) archiveToGCS(botID string) {
	log.Printf("Archiving bot %s to GCS...", botID)

	// 1. Get bot details for uid and download URL
	botDoc, err := h.botRepo.GetBotByID(context.Background(), botID)
	if err != nil {
		log.Printf("Archive failed: could not find bot %s in Firestore: %v", botID, err)
		return
	}

	uid, _ := botDoc["uid"].(string)
	if uid == "" {
		uid = "unknown"
	}

	bot, err := h.recall.GetBot(botID)
	if err != nil {
		log.Printf("Archive failed: could not get bot %s from Recall: %v", botID, err)
		return
	}

	if len(bot.Recordings) == 0 {
		return
	}

	// Recall URLs expire, so we fetch a fresh one
	downloadURL := bot.Recordings[0].MediaShortcuts.VideoMixed.URL
	if downloadURL == "" {
		log.Printf("Archive failed: no download URL for bot %s", botID)
		return
	}

	// 2. Upload to GCS
	objectName := h.storage.GetPath(uid, botID)
	gcsPath, err := h.storage.UploadFromURL(context.Background(), downloadURL, objectName)
	if err != nil {
		log.Printf("Archive failed: GCS upload error for bot %s: %v", botID, err)
		return
	}

	log.Printf("Archived bot %s to %s", botID, gcsPath)

	// 3. Update Firestore with GCS Path and Duration
	durationSec := 0
	if len(bot.Recordings) > 0 {
		durationSec = bot.Recordings[0].DurationSeconds
	}

	h.botRepo.SaveBot(context.Background(), map[string]interface{}{
		"id":         botID,
		"media_path": objectName,
		"status":     "archived",
		"duration":   fmt.Sprintf("%d", durationSec),
	})

	// 4. Cleanup Recall Media
	if err := h.recall.DeleteMedia(botID); err != nil {
		log.Printf("Cleanup failed: could not delete Recall media for bot %s: %v", botID, err)
	} else {
		log.Printf("Cleaned up Recall media for bot %s", botID)
	}
}
