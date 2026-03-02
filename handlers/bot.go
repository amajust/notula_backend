package handlers

import (
	"context"
	"log"
	"notulapro-backend/recall"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RecallClient defines the interface for interacting with Recall.ai.
type RecallClient interface {
	CreateBot(meetingURL string, joinAt *time.Time) (*recall.BotResponse, error)
	GetBot(botID string) (*recall.BotResponse, error)
	LeaveBot(botID string) error
	StartAsyncTranscription(recordingID string) error
}

// BotRepository defines the interface for persisting bot data.
type BotRepository interface {
	GetActiveBotByMeetingURL(ctx context.Context, meetingURL string) (string, error)
	GetScheduledBotByMeetingURL(ctx context.Context, meetingURL string) (string, error)
	SaveBot(ctx context.Context, bot map[string]interface{}) error
}

// BotHandler holds handler methods for bot-related routes.
type BotHandler struct {
	recall RecallClient
	repo   BotRepository
}

// NewBotHandler creates a handler with a RecallClient and BotRepository.
func NewBotHandler(r RecallClient, repo BotRepository) *BotHandler {
	return &BotHandler{recall: r, repo: repo}
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
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":  "A bot is already active in this meeting URL.",
			"bot_id": botID,
		})
	}

	bot, err := h.recall.CreateBot(body.MeetingURL, nil)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	uid, ok := c.Locals("uid").(string)
	if !ok {
		uid = "unknown" // fallback if auth isn't strict yet
	}

	// Save the new bot to Firestore for tracking and deduplication
	err = h.repo.SaveBot(c.Context(), map[string]interface{}{
		"id":          bot.ID,
		"uid":         uid, // Tie the bot to the person who requested it
		"meeting_url": body.MeetingURL,
		"status":      "joining",
		"created_at":  time.Now(),
	})

	if err != nil {
		// Soft error - the bot was still created on Recall, but we failed to track it locally.
		// We'll log it but not fail the HTTP request so the user's app doesn't break.
		log.Printf("failed to save bot %s to firestore: %v", bot.ID, err)
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
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":  "A bot is already active or scheduled for this meeting URL.",
			"bot_id": botID,
		})
	}

	bot, err := h.recall.CreateBot(body.MeetingURL, &body.JoinAt)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	uid, ok := c.Locals("uid").(string)
	if !ok {
		uid = "unknown"
	}

	// Save scheduled bot to Firestore
	err = h.repo.SaveBot(c.Context(), map[string]interface{}{
		"id":          bot.ID,
		"uid":         uid,
		"meeting_url": body.MeetingURL,
		"status":      "scheduled",
		"created_at":  time.Now(),
		"join_at":     body.JoinAt,
	})

	if err != nil {
		log.Printf("failed to save scheduled bot %s to firestore: %v", bot.ID, err)
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
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": err.Error(),
		})
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
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{"message": "bot is leaving the call"})
}

// StartTranscript godoc
// POST /recording/:id/transcript
// Trigger Gladia async transcription for a completed recording.
func (h *BotHandler) StartTranscript(c *fiber.Ctx) error {
	recordingID := c.Params("id")
	if recordingID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "recording id is required",
		})
	}

	if err := h.recall.StartAsyncTranscription(recordingID); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// TODO: update Firestore record with transcription status

	return c.JSON(fiber.Map{"message": "async transcription started"})
}
