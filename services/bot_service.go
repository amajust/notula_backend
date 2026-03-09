package services

import (
	"context"
	"log"
	"notulapro-backend/handlers"
	"time"
)

type BotService struct {
	recall handlers.RecallClient
	repo   handlers.BotRepository
}

func NewBotService(recall handlers.RecallClient, repo handlers.BotRepository) *BotService {
	return &BotService{
		recall: recall,
		repo:   repo,
	}
}

func (s *BotService) ScheduleBot(ctx context.Context, uid string, userName string, meetingURL string, joinAt time.Time) (string, error) {
	// 1. Conflict Check
	latestBot, err := s.repo.GetLatestBotByMeetingURL(ctx, meetingURL)
	if err == nil && latestBot != nil {
		status, _ := latestBot["status"].(string)
		botID, _ := latestBot["id"].(string)

		activeStatuses := map[string]bool{
			"scheduled":                    true,
			"joining":                      true,
			"in_call_recording":            true,
			"in_call_not_recording":        true,
			"in_waiting_room":              true,
			"joining_call":                 true,
			"recording_permission_allowed": true,
		}

		if activeStatuses[status] {
			return botID, handlers.ErrBotAlreadyExists
		}
	}

	// 2. Prepare Name
	botName := "Notbot"
	if userName != "" {
		botName = "Notbot on behalf of " + userName
	}

	// 3. Create at Recall
	bot, err := s.recall.CreateBot(meetingURL, botName, &joinAt)
	if err != nil {
		return "", err
	}

	// 4. Handle Terminal Replacement
	if latestBot != nil {
		status, _ := latestBot["status"].(string)
		subCode, _ := latestBot["sub_code"].(string)
		isTerminal := status == "completed" || status == "failed" || status == "fatal" ||
			status == "call_ended" || status == "cancelled" || status == "done" ||
			subCode == "timeout_exceeded_waiting_room"

		if isTerminal {
			oldID, _ := latestBot["id"].(string)
			if oldID != "" {
				log.Printf("[BotService] Replacing old terminal bot %s for scheduled URL %s", oldID, meetingURL)
				_ = s.repo.DeleteBotLocally(ctx, oldID)
			}
		}
	}

	// 5. Save to Firestore
	err = s.repo.SaveBot(ctx, map[string]interface{}{
		"id":                bot.ID,
		"uid":               uid,
		"meeting_url":       meetingURL,
		"status":            "scheduled",
		"processing_status": "Scheduled",
		"type":              "virtual",
		"title":             "Scheduled: " + meetingURL,
		"tags":              []string{"Scheduled"},
		"createdAt":         time.Now(),
		"join_at":           joinAt,
	})

	if err != nil {
		log.Printf("[BotService] Error saving bot to firestore: %v", err)
	}

	return bot.ID, nil
}
