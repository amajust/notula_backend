package middleware

import (
	"context"
	"strings"

	"firebase.google.com/go/v4/auth"
	"github.com/gofiber/fiber/v2"
)

// FirebaseAuth returns a Gofiber middleware that validates Firebase ID tokens.
// Pass in the Firebase Auth client initialized in main.go.
//
// Expected header: Authorization: Bearer <firebase-id-token>
//
// On success, stores the verified UID as c.Locals("uid") for downstream use.
func FirebaseAuth(client *auth.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing Authorization header",
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid Authorization format — expected: Bearer <token>",
			})
		}

		token, err := client.VerifyIDToken(context.Background(), parts[1])
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid or expired Firebase token",
			})
		}

		// Store UID so handlers can use it (e.g. for Firestore scoping)
		c.Locals("uid", token.UID)

		if name, ok := token.Claims["name"].(string); ok {
			c.Locals("name", name)
		}

		return c.Next()
	}
}

// GetUID retrieves the authenticated Firebase UID from the request context.
func GetUID(c *fiber.Ctx) string {
	uid, _ := c.Locals("uid").(string)
	return uid
}
