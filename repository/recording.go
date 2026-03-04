package repository

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
)

type FirestoreRecordingRepository struct {
	client *firestore.Client
}

func NewFirestoreRecordingRepository(client *firestore.Client) *FirestoreRecordingRepository {
	return &FirestoreRecordingRepository{client: client}
}

func (r *FirestoreRecordingRepository) SaveRecording(ctx context.Context, recording map[string]interface{}) error {
	id, ok := recording["id"].(string)
	if !ok || id == "" {
		log.Printf("ERROR: SaveRecording called with invalid or missing ID")
		return nil
	}
	log.Printf("Attempting to save recording to Firestore: id=%s, uid=%v", id, recording["uid"])
	_, err := r.client.Collection("recordings").Doc(id).Set(ctx, recording)
	if err != nil {
		log.Printf("ERROR: Firestore Set failed: %v", err)
	} else {
		log.Printf("SUCCESS: Recording saved to Firestore: id=%s", id)
	}
	return err
}

func (r *FirestoreRecordingRepository) UpdateRecording(ctx context.Context, id string, updates []firestore.Update) error {
	_, err := r.client.Collection("recordings").Doc(id).Update(ctx, updates)
	return err
}
