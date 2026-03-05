package events

import (
	"context"
	"fmt"
	"log"
	"strings"
)

type TranscriptEventProcessor struct {
	BotRepo BotRepository
	Recall  RecallClient
	Storage GCSClient
}

func NewTranscriptEventProcessor(br BotRepository, r RecallClient, s GCSClient) *TranscriptEventProcessor {
	return &TranscriptEventProcessor{BotRepo: br, Recall: r, Storage: s}
}

func (p *TranscriptEventProcessor) Process(ctx context.Context, event string, botID string, recordingID string) error {
	log.Printf("[TranscriptEvent] Processing %s for bot %s (recording %s)", event, botID, recordingID)

	if event != "transcript.done" {
		// Sync the status to Firestore for lifecycle visibility
		statusCode := event
		if strings.HasPrefix(event, "transcript.") {
			statusCode = strings.TrimPrefix(event, "transcript.")
		}
		return p.BotRepo.UpdateBotStatusAndSubCode(ctx, botID, statusCode, "")
	}

	if recordingID == "" {
		// Fallback: get bot to find recording ID
		bot, err := p.Recall.GetBot(botID)
		if err == nil && len(bot.Recordings) > 0 {
			recordingID = bot.Recordings[0].ID
		}
	}

	if recordingID == "" {
		return fmt.Errorf("missing recording ID for bot %s", botID)
	}

	// 1. Fetch transcript from Recall
	log.Printf("[TranscriptEvent] Fetching transcript for recording %s (bot %s)...", recordingID, botID)
	transcript, err := p.Recall.GetTranscript(recordingID)
	if err != nil {
		return fmt.Errorf("failed to fetch transcript for recording %s: %w", recordingID, err)
	}
	log.Printf("[TranscriptEvent] Successfully fetched %d transcript elements for recording %s", len(transcript), recordingID)

	// 2. Persist transcript to Firestore
	if err := p.BotRepo.SaveTranscript(ctx, botID, transcript); err != nil {
		return fmt.Errorf("failed to save transcript: %w", err)
	}

	// 3. Archive recording to GCS in background
	go p.ArchiveToGCS(botID)

	return nil
}

func (p *TranscriptEventProcessor) ArchiveToGCS(botID string) {
	log.Printf("Archiving bot %s to GCS...", botID)

	// 1. Get bot details for uid
	botDoc, err := p.BotRepo.GetBotByID(context.Background(), botID)
	if err != nil {
		log.Printf("Archive failed: could not find bot %s in Firestore: %v", botID, err)
		return
	}

	uid, _ := botDoc["uid"].(string)
	if uid == "" {
		uid = "unknown"
	}

	// 2. Get bot from Recall to get fresh download URL
	bot, err := p.Recall.GetBot(botID)
	if err != nil {
		log.Printf("Archive failed: could not get bot %s from Recall: %v", botID, err)
		return
	}

	if len(bot.Recordings) == 0 {
		return
	}

	downloadURL := bot.Recordings[0].MediaShortcuts.VideoMixed.URL
	if downloadURL == "" {
		log.Printf("Archive failed: no download URL for bot %s", botID)
		return
	}

	// 3. Upload to GCS
	objectName := p.Storage.GetPath(uid, botID)
	_, err = p.Storage.UploadFromURL(context.Background(), downloadURL, objectName)
	if err != nil {
		log.Printf("Archive failed: GCS upload error for bot %s: %v", botID, err)
		return
	}

	// 4. Update Firestore with GCS Path and mark as archived
	durationSec := 0
	if len(bot.Recordings) > 0 {
		durationSec = bot.Recordings[0].DurationSeconds
	}

	p.BotRepo.SaveBot(context.Background(), map[string]interface{}{
		"id":         botID,
		"media_path": objectName,
		"status":     "archived",
		"duration":   fmt.Sprintf("%d", durationSec),
	})

	// 5. Cleanup Recall Media
	if err := p.Recall.DeleteMedia(botID); err != nil {
		log.Printf("Cleanup failed: could not delete Recall media for bot %s: %v", botID, err)
	} else {
		log.Printf("Cleaned up Recall media for bot %s", botID)
	}
}
