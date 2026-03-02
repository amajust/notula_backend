package repository

import (
	"context"

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
		return nil
	}
	_, err := r.client.Collection("recordings").Doc(id).Set(ctx, recording)
	return err
}

func (r *FirestoreRecordingRepository) UpdateRecording(ctx context.Context, id string, updates []firestore.Update) error {
	_, err := r.client.Collection("recordings").Doc(id).Update(ctx, updates)
	return err
}
