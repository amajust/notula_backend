package handlers

import (
	"context"
	"log"

	"notulapro-backend/recall/events"

	"github.com/gofiber/fiber/v2"
)

type TranscriptHandler struct {
	Processor *events.TranscriptEventProcessor
}

func NewTranscriptHandler(p *events.TranscriptEventProcessor) *TranscriptHandler {
	return &TranscriptHandler{Processor: p}
}

func (h *TranscriptHandler) Handle(c *fiber.Ctx) error {
	var payload struct {
		Event string `json:"event"`
		Data  struct {
			Bot struct {
				ID string `json:"id"`
			} `json:"bot"`
			Recording struct {
				ID string `json:"id"`
			} `json:"recording"`
			Transcript struct {
				ID string `json:"id"`
			} `json:"transcript"`
			RecordingID string `json:"recording_id"`
		} `json:"data"`
	}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	botID := payload.Data.Bot.ID
	recordingID := payload.Data.Recording.ID
	if recordingID == "" {
		recordingID = payload.Data.RecordingID
	}
	transcriptID := payload.Data.Transcript.ID

	log.Printf("[TranscriptHandler] Received %s for bot %s (recording: %s, transcript: %s)", payload.Event, botID, recordingID, transcriptID)

	err := h.Processor.Process(context.Background(), payload.Event, botID, recordingID, transcriptID)
	if err != nil {
		log.Printf("[TranscriptHandler] Error processing transcript: %v", err)
	}

	return c.SendStatus(fiber.StatusOK)
}
