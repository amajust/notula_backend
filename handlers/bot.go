package handlers

import (
	"log"
	"time"

	"notulapro-backend/recall"

	"cloud.google.com/go/firestore"

	"github.com/gofiber/fiber/v2"
)

// BotHandler holds handler methods for bot-related routes.
type BotHandler struct {
	recall    *recall.Client
	firestore *firestore.Client
}

// NewBotHandler creates a handler with an initialized Recall.ai client and Firestore client.
func NewBotHandler(r *recall.Client, fs *firestore.Client) *BotHandler {
	return &BotHandler{recall: r, firestore: fs}
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
	// Note: You can optionally filter this by active statuses (e.g., "joining", "in_call")
	// using Firestore queries to ensure we don't block subsequent meetings on the same static link.
	iter := h.firestore.Collection("bots").
		Where("meeting_url", "==", body.MeetingURL).
		Where("status", "in", []string{"joining", "in_call_recording", "in_call_not_recording"}).
		Limit(1).
		Documents(c.Context())

	doc, err := iter.Next()
	if err == nil && doc.Exists() {
		// An active bot already exists for this URL
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":  "A bot is already active in this meeting URL.",
			"bot_id": doc.Data()["id"],
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
	_, err = h.firestore.Collection("bots").Doc(bot.ID).Set(c.Context(), map[string]interface{}{
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
	iter := h.firestore.Collection("bots").
		Where("meeting_url", "==", body.MeetingURL).
		Where("status", "in", []string{"scheduled", "joining", "in_call_recording", "in_call_not_recording"}).
		Limit(1).
		Documents(c.Context())

	doc, err := iter.Next()
	if err == nil && doc.Exists() {
		// A bot is already attached to this URL
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":  "A bot is already active or scheduled for this meeting URL.",
			"bot_id": doc.Data()["id"],
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
	_, err = h.firestore.Collection("bots").Doc(bot.ID).Set(c.Context(), map[string]interface{}{
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
