package handlers

import (
	"context"
	"encoding/json"
	"log"
	"notulapro-backend/recall/events"
	"notulapro-backend/storage"
	"notulapro-backend/utils"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RecallClient defines the interface for interacting with Recall.ai.
type RecallClient interface {
	CreateBot(meetingURL string, botName string, joinAt *time.Time) (*events.BotResponse, error)
	GetBot(botID string) (*events.BotResponse, error)
	LeaveBot(botID string) error
	DeleteBot(botID string) error
	StartAsyncTranscription(recordingID string) error
	GetTranscript(transcriptID string) ([]events.TranscriptElement, error)
	DeleteMedia(botID string) error
	SendChatMessage(botID string, text string) error
}

// BotRepository defines the interface for persisting bot data.
type BotRepository interface {
	GetActiveBotByMeetingURL(ctx context.Context, meetingURL string) (string, error)
	GetScheduledBotByMeetingURL(ctx context.Context, meetingURL string) (string, error)
	GetLatestBotByMeetingURL(ctx context.Context, meetingURL string) (map[string]interface{}, error)
	GetBotByID(ctx context.Context, botID string) (map[string]interface{}, error)
	SaveBot(ctx context.Context, bot map[string]interface{}) error
	UpdateBotStatus(ctx context.Context, botID string, status string) error
	UpdateBotStatusAndSubCode(ctx context.Context, botID string, status string, subCode string) error
	SaveTranscript(ctx context.Context, botID string, transcript interface{}) error
	DeleteBotLocally(ctx context.Context, botID string) error
}

// BotHandler holds handler methods for bot-related routes.
type BotHandler struct {
	recall  RecallClient
	repo    BotRepository
	storage *storage.GCSClient
}

// NewBotHandler creates a handler with dependencies.
func NewBotHandler(r RecallClient, repo BotRepository, s *storage.GCSClient) *BotHandler {
	return &BotHandler{recall: r, repo: repo, storage: s}
}

// ─── Request bodies ───────────────────────────────────────────────────────────

type sendBotBody struct {
	MeetingURL string `json:"meeting_url"`
}

type scheduleBotBody struct {
	MeetingURL string    `json:"meeting_url"`
	JoinAt     time.Time `json:"join_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// SendBot godoc
// POST /bot/send
// Send Notbot to a meeting immediately.
func (h *BotHandler) SendBot(c *fiber.Ctx) error {
	var body sendBotBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if body.MeetingURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "meeting_url is required",
		})
	}

	// Check if a bot already exists for this exact meeting URL
	latestBot, err := h.repo.GetLatestBotByMeetingURL(c.Context(), body.MeetingURL)
	if err == nil && latestBot != nil {
		status, _ := latestBot["status"].(string)
		botID, _ := latestBot["id"].(string)

		// If active, return Conflict
		activeStatuses := map[string]bool{
			"joining":                      true,
			"in_call_recording":            true,
			"in_call_not_recording":        true,
			"in_waiting_room":              true,
			"joining_call":                 true,
			"recording_permission_allowed": true,
		}

		if activeStatuses[status] {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error":  "A bot is already active in this meeting.",
				"bot_id": botID,
			})
		}
	}

	botName := "Notbot"
	if name, ok := c.Locals("name").(string); ok && name != "" {
		botName = "Notbot on behalf of " + name
	}

	bot, err := h.recall.CreateBot(body.MeetingURL, botName, nil)
	if err != nil {
		return utils.HandleError(c, fiber.StatusBadGateway, "Failed to connect to the recording service. Please check the meeting URL or try again later.", err)
	}
	log.Printf("[BotHandler] SendBot: Created bot with ID %s for URL %s", bot.ID, body.MeetingURL)

	uid, ok := c.Locals("uid").(string)
	if !ok {
		uid = "unknown"
	}

	// If we have a terminal bot for the same URL, we delete it to maintain "One card per meeting"
	// This satisfies the "doesn't need to create new card" requirement for retries/join-again.
	if latestBot != nil {
		status, _ := latestBot["status"].(string)
		subCode, _ := latestBot["sub_code"].(string)
		isTerminal := status == "completed" || status == "failed" || status == "fatal" ||
			status == "call_ended" || status == "cancelled" || status == "done" ||
			subCode == "timeout_exceeded_waiting_room"

		if isTerminal {
			oldID, _ := latestBot["id"].(string)
			if oldID != "" {
				log.Printf("[BotHandler] Replacing old terminal bot %s for URL %s", oldID, body.MeetingURL)
				_ = h.repo.DeleteBotLocally(c.Context(), oldID)
			}
		}
	}

	err = h.repo.SaveBot(c.Context(), map[string]interface{}{
		"id":                bot.ID,
		"uid":               uid,
		"meeting_url":       body.MeetingURL,
		"status":            "joining",
		"processing_status": "Bot is connecting...",
		"type":              "virtual",
		"title":             "Virtual Meeting " + time.Now().Format("2006-01-02 15:04"),
		"tags":              []string{"Meeting"},
		"createdAt":         time.Now(),
	})

	if err != nil {
		utils.HandleError(c, fiber.StatusInternalServerError, "Soft Error: Failed to save bot locally", err)
	}

	return c.Status(fiber.StatusCreated).JSON(bot)
}

// ScheduleBot godoc
// POST /bot/schedule
// Schedule Notbot to join a meeting at a future time (must be >10 min away).
func (h *BotHandler) ScheduleBot(c *fiber.Ctx) error {
	var body scheduleBotBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if body.MeetingURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "meeting_url is required",
		})
	}
	if body.JoinAt.IsZero() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "join_at is required",
		})
	}
	if time.Until(body.JoinAt) < 10*time.Minute {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "join_at must be at least 10 minutes in the future",
		})
	}

	// Check if a scheduled bot already exists for this exact meeting URL
	latestBot, err := h.repo.GetLatestBotByMeetingURL(c.Context(), body.MeetingURL)
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
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error":  "A bot is already active or scheduled for this meeting.",
				"bot_id": botID,
			})
		}
	}

	botName := "Notbot"
	if name, ok := c.Locals("name").(string); ok && name != "" {
		botName = "Notbot on behalf of " + name
	}

	bot, err := h.recall.CreateBot(body.MeetingURL, botName, &body.JoinAt)
	if err != nil {
		return utils.HandleError(c, fiber.StatusBadGateway, "Failed to schedule the recording bot. Please check the meeting URL and try again.", err)
	}

	uid, ok := c.Locals("uid").(string)
	if !ok {
		uid = "unknown"
	}

	// If we have a terminal bot, we "re-use" the record by replacing it
	if latestBot != nil { // Renamed from existingBot to latestBot to match the variable name above
		status, _ := latestBot["status"].(string)
		subCode, _ := latestBot["sub_code"].(string)
		isTerminal := status == "completed" || status == "failed" || status == "fatal" ||
			status == "call_ended" || status == "cancelled" || status == "done" ||
			subCode == "timeout_exceeded_waiting_room"

		if isTerminal {
			oldID, _ := latestBot["id"].(string)
			if oldID != "" {
				log.Printf("[BotHandler] Replacing old terminal bot %s for scheduled URL %s", oldID, body.MeetingURL)
				_ = h.repo.DeleteBotLocally(c.Context(), oldID)
			}
		}
	}

	// Save scheduled bot to Firestore
	err = h.repo.SaveBot(c.Context(), map[string]interface{}{
		"id":                bot.ID,
		"uid":               uid,
		"meeting_url":       body.MeetingURL,
		"status":            "scheduled",
		"processing_status": "Scheduled",
		"type":              "virtual",
		"title":             "Scheduled: " + body.MeetingURL,
		"tags":              []string{"Scheduled"},
		"createdAt":         time.Now(),
		"join_at":           body.JoinAt,
	})

	if err != nil {
		utils.HandleError(c, fiber.StatusInternalServerError, "Soft Error: Failed to save scheduled bot to firestore", err)
	}

	return c.Status(fiber.StatusCreated).JSON(bot)
}

// GetBotStatus godoc
// GET /bot/:id
// Fetch current status of a bot.
func (h *BotHandler) GetBotStatus(c *fiber.Ctx) error {
	botID := c.Params("id")
	if botID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bot id is required",
		})
	}

	bot, err := h.recall.GetBot(botID)
	if err != nil {
		return utils.HandleError(c, fiber.StatusBadGateway, "Failed to fetch bot status", err)
	}

	return c.JSON(bot)
}

// LeaveBot godoc
// POST /bot/:id/leave
// Force Notbot to leave the meeting.
func (h *BotHandler) LeaveBot(c *fiber.Ctx) error {
	botID := c.Params("id")
	if botID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bot id is required",
		})
	}

	if err := h.recall.LeaveBot(botID); err != nil {
		return utils.HandleError(c, fiber.StatusBadGateway, "Failed to tell the bot to leave the meeting", err)
	}

	return c.JSON(fiber.Map{"message": "bot is leaving the call"})
}

// DeleteBot godoc
// DELETE /bot/:id
// Cancel a bot that is still connecting or in lobby.
func (h *BotHandler) DeleteBot(c *fiber.Ctx) error {
	botID := c.Params("id")
	if botID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bot id is required"})
	}

	// 1. Tell Recall to leave the call first (safe for all states)
	_ = h.recall.LeaveBot(botID) // Ignore error if it's not in a call

	// 2. Tell Recall to delete the bot (only works for scheduled/unjoined bots)
	err := h.recall.DeleteBot(botID)
	if err != nil {
		// If Recall says 405, it's a non-scheduled bot that has joined.
		// We already called LeaveBot, so it's practically cancelled.
		if strings.Contains(err.Error(), "recall_status:405") || strings.Contains(err.Error(), "cannot_delete_bot") {
			log.Printf("[BotHandler] Bot %s cannot be deleted (405), but LeaveBot was called. Proceeding.", botID)
		} else if strings.Contains(err.Error(), "recall_status:404") {
			log.Printf("[BotHandler] Bot %s already gone (404). Proceeding.", botID)
		} else {
			return utils.HandleError(c, fiber.StatusBadGateway, "Failed to delete bot from Recall", err)
		}
	}

	// 3. Delete from firestore entirely on "Cancel"
	err = h.repo.DeleteBotLocally(c.Context(), botID)
	if err != nil {
		log.Printf("[BotHandler] Soft Error: Failed to delete record from Firestore for cancelled bot %s: %v", botID, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// StartTranscript godoc
// ...
func (h *BotHandler) StartTranscript(c *fiber.Ctx) error {
	// ... existing code ...
	return c.JSON(fiber.Map{"message": "async transcription started"})
}

// GetRecordingURL godoc
// GET /recording/:id/url
// Get a secure signed URL for a recording.
func (h *BotHandler) GetRecordingURL(c *fiber.Ctx) error {
	botID := c.Params("id")
	uid, ok := c.Locals("uid").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	// 1. Verify ownership
	bot, err := h.repo.GetBotByID(c.Context(), botID)
	if err != nil {
		return utils.HandleError(c, fiber.StatusNotFound, "Recording not found", err)
	}

	if bot["uid"] != uid {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "you do not have permission to access this recording"})
	}

	// 2. Check if archived or still on Recall
	if bot["status"] == "archived" {
		mediaPath, _ := bot["media_path"].(string)
		if mediaPath == "" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "media path not found"})
		}

		signedURL, err := h.storage.GenerateSignedURL(mediaPath, 15*time.Minute)
		if err != nil {
			return utils.HandleError(c, fiber.StatusInternalServerError, "Failed to generate signed url", err)
		}
		return c.JSON(fiber.Map{"url": signedURL})
	}

	// Fallback to Recall if not yet archived
	recallBot, err := h.recall.GetBot(botID)
	if err != nil || len(recallBot.Recordings) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "recording not available yet"})
	}

	return c.JSON(fiber.Map{"url": recallBot.Recordings[0].MediaShortcuts.VideoMixed.Data.DownloadURL})
}

// GetBotTranscript godoc
// GET /bot/:id/transcript
// Fetch transcript for a bot, with fallback to Recall if not in Firestore.
func (h *BotHandler) GetBotTranscript(c *fiber.Ctx) error {
	botID := c.Params("id")
	log.Printf("[BotHandler] GetBotTranscript: Received request for bot %s", botID)
	if botID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bot id is required"})
	}

	// 1. Check Firestore first
	botDoc, err := h.repo.GetBotByID(c.Context(), botID)
	if err == nil {
		if transcript, ok := botDoc["transcript"]; ok && transcript != nil {
			// Basic validation: check if it's just dummy data [{Start:0, End:0, Text:"", Speaker:""}]
			tList, isList := transcript.([]interface{})
			isValid := isList && len(tList) > 0
			if isList && len(tList) == 1 {
				first, isMap := tList[0].(map[string]interface{})
				if isMap {
					text, _ := first["text"].(string)
					if text == "" {
						text, _ = first["Text"].(string)
					}
					speaker, _ := first["speaker"].(string)
					if speaker == "" {
						speaker, _ = first["Speaker"].(string)
					}

					if text == "" && (speaker == "" || speaker == "Unknown") {
						isValid = false
						log.Printf("[BotHandler] Firestore transcript for bot %s is dummy data, fetching from Recall", botID)
					}
				}
			}

			if isValid {
				return c.JSON(fiber.Map{"transcript": transcript})
			}
		}
	}

	// 2. Fallback: Fetch from Recall
	recallBot, err := h.recall.GetBot(botID)
	if err != nil {
		return utils.HandleError(c, fiber.StatusBadGateway, "Failed to fetch bot from Recall", err)
	}

	botJSON, _ := json.MarshalIndent(recallBot, "", "  ")
	log.Printf("[BotHandler] Recall Bot Response for %s:\n%s", botID, string(botJSON))

	// Find the first "done" transcript (check top-level first, then recordings shortcuts)
	var transcriptID string
	for _, t := range recallBot.Transcripts {
		if t.Status.Code == "done" {
			transcriptID = t.ID
			break
		}
	}

	if transcriptID == "" {
		for _, rec := range recallBot.Recordings {
			if rec.MediaShortcuts.Transcript.Status.Code == "done" && rec.MediaShortcuts.Transcript.ID != "" {
				transcriptID = rec.MediaShortcuts.Transcript.ID
				log.Printf("[BotHandler] Found 'done' transcript %s in recording shortcut for bot %s", transcriptID, botID)
				break
			}
		}
	}

	if transcriptID != "" {
		log.Printf("[BotHandler] Using 'done' transcript %s for bot %s", transcriptID, botID)
	}

	if transcriptID == "" {
		// If no transcript is "done", check if any are processing
		for _, t := range recallBot.Transcripts {
			if t.Status.Code == "processing" {
				return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"error": "transcript is still processing"})
			}
		}
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "transcript not available yet"})
	}

	// Fetch transcript data
	transcript, err := h.recall.GetTranscript(transcriptID)
	if err != nil {
		return utils.HandleError(c, fiber.StatusBadGateway, "Failed to fetch transcript data from Recall", err)
	}

	// Save to Firestore for future
	if err := h.repo.SaveTranscript(c.Context(), botID, transcript); err != nil {
		log.Printf("[BotHandler] Error saving transcript to Firestore for bot %s: %v", botID, err)
	}

	return c.JSON(fiber.Map{"transcript": transcript})
}
