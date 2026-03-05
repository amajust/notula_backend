package events

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

type TranscriptEventProcessor struct {
	BotRepo BotRepository
	Recall  RecallClient
	Storage GCSClient
}

func NewTranscriptEventProcessor(br BotRepository, r RecallClient, s GCSClient) *TranscriptEventProcessor {
	return &TranscriptEventProcessor{BotRepo: br, Recall: r, Storage: s}
}

func (p *TranscriptEventProcessor) Process(ctx context.Context, event string, botID string, recordingID string, transcriptID string) error {
	log.Printf("[TranscriptEvent] Processing %s for bot %s (recording %s, transcript %s)", event, botID, recordingID, transcriptID)

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

	// 1. Fetch transcript from Recall using Transcript ID (Modern v2 Flow)
	if transcriptID == "" {
		// Fallback to recording ID if transcript ID is missing (should not happen for transcript.done)
		transcriptID = recordingID
	}

	log.Printf("[TranscriptEvent] Fetching transcript for ID %s (bot %s)...", transcriptID, botID)
	transcript, err := p.Recall.GetTranscript(transcriptID)
	if err != nil {
		return fmt.Errorf("failed to fetch transcript for ID %s: %w", transcriptID, err)
	}
	log.Printf("[TranscriptEvent] Successfully fetched %d transcript elements for bot %s", len(transcript), botID)

	// 2. Persist transcript to Firestore
	if err := p.BotRepo.SaveTranscript(ctx, botID, transcript); err != nil {
		return fmt.Errorf("failed to save transcript: %w", err)
	}

	// 3. Update status to recorded to indicate transcript is ready (synced with processing_status)
	if err := p.BotRepo.UpdateBotStatusAndSubCode(ctx, botID, "recorded", ""); err != nil {
		log.Printf("Failed to update status to recorded for bot %s: %v", botID, err)
	}

	// 4. Archive recording to GCS in background
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
	var downloadURL string
	var bot *BotResponse

	// Retry logic: video mixed might take a few seconds after transcript.done
	for i := 0; i < 5; i++ {
		bot, err = p.Recall.GetBot(botID)
		if err != nil {
			log.Printf("Archive attempt %d: could not get bot %s from Recall: %v", i+1, botID, err)
			time.Sleep(5 * time.Second)
			continue
		}

		if len(bot.Recordings) > 0 {
			downloadURL = bot.Recordings[0].MediaShortcuts.VideoMixed.Data.DownloadURL
			if downloadURL != "" {
				break
			}
		}

		log.Printf("Archive attempt %d: no download URL yet for bot %s, waiting...", i+1, botID)
		time.Sleep(5 * time.Second)
	}

	if downloadURL == "" {
		log.Printf("Archive failed: no download URL for bot %s after retries", botID)
		return
	}

	// 3. Upload to GCS
	objectName := p.Storage.GetPath(uid, botID)
	log.Printf("[Archive] Uploading media for bot %s to GCS: %s", botID, objectName)
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
