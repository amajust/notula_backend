package handlers

import (
	"context"
	"log"

	"notulapro-backend/recall/events"

	"github.com/gofiber/fiber/v2"
)

type RealtimeHandler struct {
	Processor *events.RealtimeEventProcessor
}

func NewRealtimeHandler(p *events.RealtimeEventProcessor) *RealtimeHandler {
	return &RealtimeHandler{Processor: p}
}

func (h *RealtimeHandler) Handle(c *fiber.Ctx) error {
	var payload struct {
		Event string `json:"event"`
		Data  struct {
			Data struct {
				Participant struct {
					Name string `json:"name"`
				} `json:"participant"`
				Words []struct {
					Text string `json:"text"`
				} `json:"words"`
			} `json:"data"`
			Bot struct {
				ID string `json:"id"`
			} `json:"bot"`
		} `json:"data"`
	}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	botID := payload.Data.Bot.ID
	speakerName := payload.Data.Data.Participant.Name
	words := payload.Data.Data.Words

	err := h.Processor.ProcessTranscript(context.Background(), payload.Event, botID, speakerName, words)
	if err != nil {
		log.Printf("[RealtimeHandler] Error processing realtime transcript: %v", err)
	}

	return c.SendStatus(fiber.StatusOK)
}
