package handlers

import (
	"log"
	"notulapro-backend/recall"
	"notulapro-backend/repository"
	"notulapro-backend/utils"

	"github.com/gofiber/fiber/v2"
)

type CalendarHandler struct {
	recall *recall.Client
	repo   *repository.FirestoreUserRepository
}

func NewCalendarHandler(r *recall.Client, repo *repository.FirestoreUserRepository) *CalendarHandler {
	return &CalendarHandler{recall: r, repo: repo}
}

// ConnectCalendar godoc
// POST /calendar/connect
// Returns the Recall-hosted connect URL. Creates a calendar object if none exists.
func (h *CalendarHandler) ConnectCalendar(c *fiber.Ctx) error {
	uid, ok := c.Locals("uid").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	profile, err := h.repo.GetProfile(c.Context(), uid)
	if err != nil {
		return utils.HandleError(c, fiber.StatusInternalServerError, "Failed to fetch user profile", err)
	}

	calendarID := profile.RecallCalendarID
	if calendarID == "" {
		log.Printf("[CalendarHandler] Creating new Recall calendar for user %s", uid)
		cal, err := h.recall.CreateCalendar()
		if err != nil {
			return utils.HandleError(c, fiber.StatusBadGateway, "Failed to create calendar in Recall", err)
		}
		calendarID = cal.ID
		if err := h.repo.SaveRecallCalendarID(c.Context(), uid, calendarID); err != nil {
			log.Printf("[CalendarHandler] Warning: Failed to save calendar ID locally: %v", err)
		}
	}

	oauthURL, err := h.recall.GetCalendarOauthURL(calendarID)
	if err != nil {
		return utils.HandleError(c, fiber.StatusBadGateway, "Failed to fetch OAuth URL from Recall", err)
	}

	return c.JSON(fiber.Map{
		"oauth_url": oauthURL,
	})
}

// GetCalendarStatus godoc
// GET /calendar/status
// Returns the status of the linked calendar.
func (h *CalendarHandler) GetCalendarStatus(c *fiber.Ctx) error {
	uid, ok := c.Locals("uid").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	profile, err := h.repo.GetProfile(c.Context(), uid)
	if err != nil {
		return utils.HandleError(c, fiber.StatusInternalServerError, "Failed to fetch user profile", err)
	}

	if profile.RecallCalendarID == "" {
		return c.JSON(fiber.Map{
			"connected": false,
		})
	}

	cal, err := h.recall.GetCalendar(profile.RecallCalendarID)
	if err != nil {
		return utils.HandleError(c, fiber.StatusBadGateway, "Failed to fetch calendar status from Recall", err)
	}

	return c.JSON(fiber.Map{
		"connected":           cal.Status == "active",
		"status":              cal.Status,
		"platform":            cal.Platform,
		"automatic_recording": cal.AutomaticRecording,
	})
}

// SyncAutoJoinPreference godoc
// PATCH /calendar/settings
// Syncs our local autoJoin preference with Recall's automatic_recording settings.
func (h *CalendarHandler) SyncAutoJoinSettings(c *fiber.Ctx) error {
	uid, ok := c.Locals("uid").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var body struct {
		IsAutoJoinEnabled bool `json:"is_enabled"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	profile, err := h.repo.GetProfile(c.Context(), uid)
	if err != nil {
		return utils.HandleError(c, fiber.StatusInternalServerError, "Failed to fetch user profile", err)
	}

	if profile.RecallCalendarID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No calendar connected"})
	}

	// Update Recall
	payload := map[string]interface{}{
		"automatic_recording": map[string]interface{}{
			"is_enabled":      body.IsAutoJoinEnabled,
			"record_external": true,
			"record_internal": true,
		},
	}
	_, err = h.recall.UpdateCalendar(profile.RecallCalendarID, payload)
	if err != nil {
		return utils.HandleError(c, fiber.StatusBadGateway, "Failed to update Recall calendar settings", err)
	}

	// Also update our local preference for UI consistency
	err = h.repo.UpdatePreferences(c.Context(), uid, repository.UserPreferences{
		AutoJoinCalendar: body.IsAutoJoinEnabled,
	})
	if err != nil {
		log.Printf("[CalendarHandler] Warning: Failed to update local autoJoin preference: %v", err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}
