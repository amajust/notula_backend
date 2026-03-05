package handlers

import (
	"context"
	"log"

	"notulapro-backend/recall/events"

	"github.com/gofiber/fiber/v2"
)

type RecordingHandler struct {
	Processor *events.RecordingEventProcessor
}

func NewRecordingHandler(p *events.RecordingEventProcessor) *RecordingHandler {
	return &RecordingHandler{Processor: p}
}

func (h *RecordingHandler) Handle(c *fiber.Ctx) error {
	var payload struct {
		Event string `json:"event"`
		Data  struct {
			Data struct {
				Code    string `json:"code"`
				SubCode string `json:"sub_code"`
			} `json:"data"`
			Bot struct {
				ID string `json:"id"`
			} `json:"bot"`
			Recording struct {
				ID string `json:"id"`
			} `json:"recording"`
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
	statusCode := payload.Data.Data.Code
	subCode := payload.Data.Data.SubCode

	log.Printf("[RecordingHandler] Received %s for bot %s (recording: %s, status: %s)", payload.Event, botID, recordingID, statusCode)

	err := h.Processor.Process(context.Background(), payload.Event, botID, recordingID, statusCode, subCode)
	if err != nil {
		log.Printf("[RecordingHandler] Error processing event: %v", err)
	}

	return c.SendStatus(fiber.StatusOK)
}
