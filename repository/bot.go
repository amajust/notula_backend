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
	iter := r.client.Collection("recordings").
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
	iter := r.client.Collection("recordings").
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

func (r *FirestoreBotRepository) GetBotByID(ctx context.Context, botID string) (map[string]interface{}, error) {
	doc, err := r.client.Collection("recordings").Doc(botID).Get(ctx)
	if err != nil {
		return nil, err
	}
	return doc.Data(), nil
}

func (r *FirestoreBotRepository) SaveBot(ctx context.Context, bot map[string]interface{}) error {
	id, ok := bot["id"].(string)
	if !ok || id == "" {
		return nil // or error
	}
	_, err := r.client.Collection("recordings").Doc(id).Set(ctx, bot, firestore.MergeAll)
	return err
}

func (r *FirestoreBotRepository) UpdateBotStatus(ctx context.Context, botID string, status string) error {
	_, err := r.client.Collection("recordings").Doc(botID).Update(ctx, []firestore.Update{
		{Path: "status", Value: status},
	})
	return err
}

func (r *FirestoreBotRepository) SaveTranscript(ctx context.Context, botID string, transcript interface{}) error {
	_, err := r.client.Collection("recordings").Doc(botID).Update(ctx, []firestore.Update{
		{Path: "transcript", Value: transcript},
		{Path: "status", Value: "completed"},
	})
	return err
}
