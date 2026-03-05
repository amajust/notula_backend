package handlers

import (
	"context"
	"notulapro-backend/recall/events"
	"notulapro-backend/storage"
	"notulapro-backend/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RecallClient defines the interface for interacting with Recall.ai.
type RecallClient interface {
	CreateBot(meetingURL string, botName string, joinAt *time.Time) (*events.BotResponse, error)
	GetBot(botID string) (*events.BotResponse, error)
	LeaveBot(botID string) error
	StartAsyncTranscription(recordingID string) error
	GetTranscript(botID string) ([]events.TranscriptElement, error)
	DeleteMedia(botID string) error
	SendChatMessage(botID string, text string) error
}

// BotRepository defines the interface for persisting bot data.
type BotRepository interface {
	GetActiveBotByMeetingURL(ctx context.Context, meetingURL string) (string, error)
	GetScheduledBotByMeetingURL(ctx context.Context, meetingURL string) (string, error)
	GetBotByID(ctx context.Context, botID string) (map[string]interface{}, error)
	SaveBot(ctx context.Context, bot map[string]interface{}) error
	UpdateBotStatus(ctx context.Context, botID string, status string) error
	UpdateBotStatusAndSubCode(ctx context.Context, botID string, status string, subCode string) error
	SaveTranscript(ctx context.Context, botID string, transcript interface{}) error
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
	botID, err := h.repo.GetActiveBotByMeetingURL(c.Context(), body.MeetingURL)
	if err == nil && botID != "" {
		// An active bot already exists for this URL
		return utils.HandleError(c, fiber.StatusConflict, "A bot is already active in this meeting URL.", err)
	}

	botName := "Notbot"
	if name, ok := c.Locals("name").(string); ok && name != "" {
		botName = "Notbot on behalf of " + name
	}

	bot, err := h.recall.CreateBot(body.MeetingURL, botName, nil)
	if err != nil {
		return utils.HandleError(c, fiber.StatusBadGateway, "Failed to connect to the recording service. Please check the meeting URL or try again later.", err)
	}

	uid, ok := c.Locals("uid").(string)
	if !ok {
		uid = "unknown" // fallback if auth isn't strict yet
	}

	// Save the new bot to Firestore for tracking and deduplication
	err = h.repo.SaveBot(c.Context(), map[string]interface{}{
		"id":                bot.ID,
		"uid":               uid, // Tie the bot to the person who requested it
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
	botID, err := h.repo.GetScheduledBotByMeetingURL(c.Context(), body.MeetingURL)
	if err == nil && botID != "" {
		// A bot is already attached to this URL
		return utils.HandleError(c, fiber.StatusConflict, "A bot is already active or scheduled for this meeting URL.", err)
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
