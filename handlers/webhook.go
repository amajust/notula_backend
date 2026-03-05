package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"notulapro-backend/storage"

	"cloud.google.com/go/firestore"
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
		h.handleTranscriptDone(c, payload.Data.BotID)
	case "gladia.transcription.done": // Custom event if we handle offline recordings
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

	// 2. We skip asynchronous Gladia trigger here because we use Real-Time transcription.
	// We just wait for the `transcript.done` webhook.

	// 3. Update status in repository
	if err := h.botRepo.UpdateBotStatus(context.Background(), botID, "recorded"); err != nil {
		log.Printf("Failed to update bot %s status: %v", botID, err)
	}
}

func (h *WebhookHandler) handleTranscriptDone(c *fiber.Ctx, botID string) {
	log.Printf("Transcript for bot %s is ready, fetching...", botID)

	// The user specified to use Recall's media_shortcuts download URL for the full transcript
	// However, we can also use recall.GetTranscript(botID) which returns []TranscriptElement
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

// RecallRealtimeWebhook handles real-time transcription utterances and injects them into the chat.
func (h *WebhookHandler) RecallRealtimeWebhook(c *fiber.Ctx) error {
	var payload struct {
		Event string `json:"event"`
		Data  struct {
			Data struct {
				Words []struct {
					Text string `json:"text"`
				} `json:"words"`
				Participant struct {
					Name string `json:"name"`
				} `json:"participant"`
			} `json:"data"`
			Bot struct {
				ID string `json:"id"`
			} `json:"bot"`
		} `json:"data"`
	}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	if payload.Event == "transcript.data" {
		botID := payload.Data.Bot.ID
		speakerName := payload.Data.Data.Participant.Name

		var fullText string
		for _, w := range payload.Data.Data.Words {
			fullText += w.Text
		}

		if speakerName == "" {
			speakerName = "Participant"
		}

		if fullText != "" {
			chatMessage := fmt.Sprintf("[%s]: %s", speakerName, fullText)
			log.Printf("Live Transcript for %s -> %s", botID, chatMessage)

			err := h.recall.SendChatMessage(botID, chatMessage)
			if err != nil {
				log.Printf("Failed to send chat message for bot %s: %v", botID, err)
			}
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

// GladiaWebhook handles incoming results from Gladia.
func (h *WebhookHandler) GladiaWebhook(c *fiber.Ctx) error {
	botID := c.Query("bot_id")
	recordingID := c.Query("recording_id") // For offline recordings

	var payload struct {
		Event string `json:"event"`
		Data  struct {
			ID            string `json:"id"`
			Status        string `json:"status"`
			Transcription any    `json:"transcription"`
			Error         string `json:"error"` // Error message if transcription failed
		} `json:"data"`
	}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	// Handle transcription completion
	if payload.Event == "transcription.done" {
		log.Printf("Gladia transcript for %s is ready, saving...", recordingID)

		// For offline recordings, update status to "completed"
		if recordingID != "" {
			updates := []firestore.Update{
				{Path: "status", Value: "completed"},
				{Path: "processingCompletedAt", Value: time.Now()},
			}

			if payload.Data.Transcription != nil {
				updates = append(updates, firestore.Update{Path: "transcript", Value: payload.Data.Transcription})
			}

			if err := h.recRepo.UpdateRecording(c.Context(), recordingID, updates); err != nil {
				log.Printf("Failed to update offline recording %s status: %v", recordingID, err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update recording status"})
			}

			log.Printf("Successfully processed offline recording %s", recordingID)
			return c.SendStatus(fiber.StatusOK)
		}

		// For virtual recordings (bot-based), use existing logic
		if err := h.botRepo.SaveTranscript(c.Context(), botID, payload.Data.Transcription); err != nil {
			log.Printf("Failed to save Gladia transcript for bot %s: %v", botID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save transcript"})
		}

		// Archive to GCS in background
		go h.archiveToGCS(botID)

		return c.SendStatus(fiber.StatusOK)
	}

	// Handle transcription failure
	if payload.Event == "transcription.failed" || payload.Data.Status == "failed" {
		errorMsg := payload.Data.Error
		if errorMsg == "" {
			errorMsg = "Transcription failed without error message"
		}

		log.Printf("Gladia transcription failed for recording %s: %s", recordingID, errorMsg)

		// For offline recordings, update status to "failed"
		if recordingID != "" {
			updates := []firestore.Update{
				{Path: "status", Value: "failed"},
				{Path: "uploadError", Value: errorMsg},
				{Path: "processingCompletedAt", Value: time.Now()},
			}

			if err := h.recRepo.UpdateRecording(c.Context(), recordingID, updates); err != nil {
				log.Printf("Failed to update offline recording %s status to failed: %v", recordingID, err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update recording status"})
			}

			log.Printf("Marked offline recording %s as failed", recordingID)
			return c.SendStatus(fiber.StatusOK)
		}

		// For virtual recordings (bot-based), update bot status
		if botID != "" {
			if err := h.botRepo.UpdateBotStatus(c.Context(), botID, "failed"); err != nil {
				log.Printf("Failed to update bot %s status to failed: %v", botID, err)
			}
			log.Printf("Marked bot %s as failed", botID)
			return c.SendStatus(fiber.StatusOK)
		}
	}

	log.Printf("Gladia webhook event %s - ignoring", payload.Event)
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
