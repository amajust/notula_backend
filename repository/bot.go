package repository

import (
	"context"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

type FirestoreBotRepository struct {
	client *firestore.Client
}

func NewFirestoreBotRepository(client *firestore.Client) *FirestoreBotRepository {
	return &FirestoreBotRepository{client: client}
}

func (r *FirestoreBotRepository) GetActiveBotByMeetingURL(ctx context.Context, meetingURL string) (string, error) {
	iter := r.client.Collection("bots").
		Where("meeting_url", "==", meetingURL).
		Where("status", "in", []string{"joining", "in_call_recording", "in_call_not_recording"}).
		Limit(1).
		Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return doc.Data()["id"].(string), nil
}

func (r *FirestoreBotRepository) GetScheduledBotByMeetingURL(ctx context.Context, meetingURL string) (string, error) {
	iter := r.client.Collection("bots").
		Where("meeting_url", "==", meetingURL).
		Where("status", "in", []string{"scheduled", "joining", "in_call_recording", "in_call_not_recording"}).
		Limit(1).
		Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return doc.Data()["id"].(string), nil
}

func (r *FirestoreBotRepository) SaveBot(ctx context.Context, bot map[string]interface{}) error {
	id, ok := bot["id"].(string)
	if !ok || id == "" {
		return nil // or error
	}
	_, err := r.client.Collection("bots").Doc(id).Set(ctx, bot)
	return err
}
