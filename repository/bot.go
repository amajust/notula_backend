package repository

import (
	"context"
	"log"

	"notulapro-backend/utils"

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

func (r *FirestoreBotRepository) GetLatestBotByMeetingURL(ctx context.Context, meetingURL string) (map[string]interface{}, error) {
	iter := r.client.Collection("recordings").
		Where("meeting_url", "==", meetingURL).
		OrderBy("createdAt", firestore.Desc).
		Limit(1).
		Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return doc.Data(), nil
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
	return r.UpdateBotStatusAndSubCode(ctx, botID, status, "")
}

func (r *FirestoreBotRepository) UpdateBotStatusAndSubCode(ctx context.Context, botID string, status string, subCode string) error {
	existing, err := r.GetBotByID(ctx, botID)
	if err == nil && existing != nil {
		currStatus, _ := existing["status"].(string)
		if currStatus == "archived" || currStatus == "completed" || currStatus == "recorded" || currStatus == "transcript.done" {
			log.Printf("[Repository] Skipping status update to %s for bot %s because it is already %s", status, botID, currStatus)
			return nil
		}

		// Preserve subCode if not provided in the current update
		if subCode == "" {
			subCode, _ = existing["sub_code"].(string)
		}
	}

	processingStatus := utils.GetFriendlyProcessingStatus(status)
	if (status == "failed" || status == "call_ended" || status == "fatal" || status == "done" || status == "processing") && subCode != "" {
		log.Printf("[Repository] Mapping subCode %s to processingStatus for status %s", subCode, status)
		processingStatus = utils.GetFriendlyRecallMessage(subCode)
	}

	data := make(map[string]interface{})
	data["status"] = status
	if subCode != "" {
		data["sub_code"] = subCode
	}
	if processingStatus != "" {
		data["processing_status"] = processingStatus
	}

	_, err = r.client.Collection("recordings").Doc(botID).Set(ctx, data, firestore.MergeAll)
	return err
}

func (r *FirestoreBotRepository) SaveTranscript(ctx context.Context, botID string, transcript interface{}) error {
	_, err := r.client.Collection("recordings").Doc(botID).Update(ctx, []firestore.Update{
		{Path: "transcript", Value: transcript},
	})
	return err
}

func (r *FirestoreBotRepository) DeleteBotLocally(ctx context.Context, botID string) error {
	_, err := r.client.Collection("recordings").Doc(botID).Delete(ctx)
	return err
}
