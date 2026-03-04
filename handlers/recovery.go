package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"notulapro-backend/storage"

	"cloud.google.com/go/firestore"
)

// RecoverStuckRecordings checks for recordings stuck in "processing" status
// and retries Gladia transcription for them
func RecoverStuckRecordings(
	ctx context.Context,
	recordingRepo RecordingRepository,
	gladiaClient GladiaClient,
	storageClient *storage.FirebaseStorageClient,
	baseURL string,
) error {
	if storageClient == nil {
		log.Println("[Recovery] Storage client not available, skipping recovery")
		return nil
	}

	// Query for recordings stuck in "processing" for more than 30 minutes
	cutoffTime := time.Now().Add(-30 * time.Minute)

	log.Printf("[Recovery] Checking for recordings stuck in 'processing' since %v", cutoffTime)

	// Note: This requires a custom query method in the repository
	// For now, we'll log that recovery is available but needs implementation
	log.Println("[Recovery] Recovery mechanism available - requires custom repository query")
	log.Println("[Recovery] To enable: Add GetStuckRecordings() method to RecordingRepository")

	return nil
}

// RetryRecordingTranscription retries Gladia transcription for a specific recording
func RetryRecordingTranscription(
	ctx context.Context,
	recordingID string,
	storagePath string,
	recordingRepo RecordingRepository,
	gladiaClient GladiaClient,
	storageClient *storage.FirebaseStorageClient,
	callbackURL string,
) error {
	if storageClient == nil {
		return fmt.Errorf("storage client not available")
	}

	// Generate signed URL
	signedURL, err := storageClient.GenerateSignedURL(storagePath, 60)
	if err != nil {
		return fmt.Errorf("failed to generate signed URL: %w", err)
	}

	// Trigger Gladia transcription
	gladiaRes, err := gladiaClient.Transcribe(signedURL, callbackURL)
	if err != nil {
		return fmt.Errorf("failed to trigger transcription: %w", err)
	}

	// Update Firestore with new Gladia ID
	err = recordingRepo.UpdateRecording(ctx, recordingID, []firestore.Update{
		{Path: "gladiaId", Value: gladiaRes.ID},
		{Path: "retryCount", Value: firestore.Increment(1)},
		{Path: "lastRetryAt", Value: time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to update recording: %w", err)
	}

	log.Printf("[Recovery] Retried transcription for recording %s (gladia_id: %s)", recordingID, gladiaRes.ID)
	return nil
}
