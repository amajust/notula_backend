package events

import (
	"context"
	"log"
	"strings"
)

type BotEventProcessor struct {
	BotRepo BotRepository
}

func NewBotEventProcessor(br BotRepository) *BotEventProcessor {
	return &BotEventProcessor{BotRepo: br}
}

func (p *BotEventProcessor) Process(ctx context.Context, event string, botID string, statusCode string, subCode string) error {
	// In V2, event names are often "bot." + statusCode (like bot.joining_call)
	// This covers events like: bot.joining_call, bot.in_waiting_room, bot.in_call_recording,
	// bot.call_ended, bot.fatal, bot.done, and breakout room events:
	// bot.breakout_room_opened, bot.breakout_room_entered, etc.
	if statusCode == "" && strings.HasPrefix(event, "bot.") && event != "bot.status_change" {
		statusCode = strings.TrimPrefix(event, "bot.")
	}

	if botID != "" && statusCode != "" {
		log.Printf("[BotEvent] Processing %s (mapped status: %s) for bot %s", event, statusCode, botID)
		return p.BotRepo.UpdateBotStatusAndSubCode(ctx, botID, statusCode, subCode)
	}
	return nil
}
