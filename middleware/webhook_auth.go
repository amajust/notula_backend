package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// RecallWebhookAuth validates the HMAC signature sent by Recall.ai
func RecallWebhookAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		secret := os.Getenv("RECALL_WEBHOOK_SECRET")
		if secret == "" {
			log.Println("❌ [RecallWebhookAuth] RECALL_WEBHOOK_SECRET is not configured in .env. Webhook verification failed.")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "webhook secret is not configured on the server",
			})
		}

		msgID := c.Get("webhook-id")
		if msgID == "" {
			msgID = c.Get("svix-id")
		}

		msgTimestamp := c.Get("webhook-timestamp")
		if msgTimestamp == "" {
			msgTimestamp = c.Get("svix-timestamp")
		}

		msgSignature := c.Get("webhook-signature")
		if msgSignature == "" {
			msgSignature = c.Get("svix-signature")
		}

		if msgID == "" || msgTimestamp == "" || msgSignature == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing webhook verification headers",
			})
		}

		// Process Secret (remove whsec_ prefix and base64 decode)
		prefix := "whsec_"
		base64Secret := secret
		if strings.HasPrefix(secret, prefix) {
			base64Secret = secret[len(prefix):]
		}

		key, err := base64.StdEncoding.DecodeString(base64Secret)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to decode webhook secret",
			})
		}

		// Body string
		payloadStr := string(c.Body())

		toSign := fmt.Sprintf("%s.%s.%s", msgID, msgTimestamp, payloadStr)

		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(toSign))
		expectedSig := mac.Sum(nil)

		// Compare passing signatures
		passedSigs := strings.Split(msgSignature, " ")
		for _, versionedSig := range passedSigs {
			parts := strings.SplitN(versionedSig, ",", 2)
			if len(parts) != 2 {
				continue
			}
			version, signatureBase64 := parts[0], parts[1]

			if version != "v1" {
				continue
			}

			sigBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
			if err != nil {
				continue
			}

			if len(expectedSig) == len(sigBytes) && subtle.ConstantTimeCompare(expectedSig, sigBytes) == 1 {
				// Signature is valid
				return c.Next()
			}
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid webhook signature",
		})
	}
}
