package utils

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

// HandleError logs the real, detailed system error from the backend/database/API,
// and returns a clean, user-friendly JSON message to the frontend client.
func HandleError(c *fiber.Ctx, statusCode int, userMessage string, internalErr error) error {
	// Log the actual detailed error in our backend system for debugging
	log.Printf("[ERROR] %s: %v\n", userMessage, internalErr)

	// Return only the generic message to the client
	return c.Status(statusCode).JSON(fiber.Map{
		"error": userMessage,
	})
}
