package handlers

import (
	"context"
	"log"

	"notulapro-backend/recall/events"

	"github.com/gofiber/fiber/v2"
)

type BotHandler struct {
	Processor *events.BotEventProcessor
}

func NewBotHandler(p *events.BotEventProcessor) *BotHandler {
	return &BotHandler{Processor: p}
}

func (h *BotHandler) Handle(c *fiber.Ctx) error {
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
		} `json:"data"`
	}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	botID := payload.Data.Bot.ID
	statusCode := payload.Data.Data.Code
	subCode := payload.Data.Data.SubCode

	log.Printf("[BotHandler] Received %s for bot %s (status: %s, subCode: %s)", payload.Event, botID, statusCode, subCode)

	err := h.Processor.Process(context.Background(), payload.Event, botID, statusCode, subCode)
	if err != nil {
		log.Printf("[BotHandler] Error processing event: %v", err)
	}

	return c.SendStatus(fiber.StatusOK)
}
