package handlers

import (
	"time"

	"notulapro-backend/recall"

	"github.com/gofiber/fiber/v2"
)

// BotHandler holds handler methods for bot-related routes.
type BotHandler struct {
	recall *recall.Client
}

// NewBotHandler creates a handler with an initialized Recall.ai client.
func NewBotHandler(r *recall.Client) *BotHandler {
	return &BotHandler{recall: r}
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

	bot, err := h.recall.CreateBot(body.MeetingURL, nil)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// TODO: save bot to Firestore for tracking

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

	bot, err := h.recall.CreateBot(body.MeetingURL, &body.JoinAt)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// TODO: save scheduled bot to Firestore

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
