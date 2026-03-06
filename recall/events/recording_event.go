package events

import (
	"context"
	"log"
	"strings"
)

type RecordingEventProcessor struct {
	BotRepo BotRepository
	Recall  RecallClient
}

func NewRecordingEventProcessor(br BotRepository, r RecallClient) *RecordingEventProcessor {
	return &RecordingEventProcessor{BotRepo: br, Recall: r}
}

func (p *RecordingEventProcessor) Process(ctx context.Context, event string, botID string, recordingID string, statusCode string, subCode string) error {
	// This covers events: recording.processing, recording.done, recording.failed,
	// recording.paused, recording.deleted
	if statusCode == "" && strings.HasPrefix(event, "recording.") {
		statusCode = strings.TrimPrefix(event, "recording.")
	}

	if botID != "" && statusCode != "" {
		log.Printf("[RecordingEvent] Processing %s (mapped status: %s) for bot %s (recording %s)", event, statusCode, botID, recordingID)
		err := p.BotRepo.UpdateBotStatusAndSubCode(ctx, botID, statusCode, subCode)
		if err != nil {
			log.Printf("Failed to sync bot %s recording status: %v", botID, err)
		}

		if statusCode == "done" || subCode == "recording_done" {
			p.HandleBotDone(ctx, botID)
		}
	}
	return nil
}

func (p *RecordingEventProcessor) HandleBotDone(ctx context.Context, botID string) {
	log.Printf("Bot %s finished recording, updating status...", botID)

	// 1. Fetch bot details to ensure we have information
	bot, err := p.Recall.GetBot(botID)
	if err != nil {
		log.Printf("Failed to get bot %s: %v", botID, err)
		return
	}

	if len(bot.Recordings) == 0 {
		log.Printf("No recordings found for bot %s", botID)
		return
	}

	// 2. Update status in repository to indicate media is ready
	if err := p.BotRepo.UpdateBotStatus(ctx, botID, "recording_done"); err != nil {
		log.Printf("Failed to update bot %s status: %v", botID, err)
	}

	// 3. Trigger Async Transcription
	log.Printf("Starting async transcription for recording %s...", bot.Recordings[0].ID)
	if err := p.Recall.StartAsyncTranscription(bot.Recordings[0].ID); err != nil {
		log.Printf("Failed to start async transcription for bot %s: %v", botID, err)
	}
}
