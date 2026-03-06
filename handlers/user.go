package handlers

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
)

type StorageClient interface {
	GetTotalStorageUsed(ctx context.Context, uid string) (int64, error)
}

type UserHandler struct {
	gcs      StorageClient
	firebase StorageClient
}

func NewUserHandler(gcs StorageClient, firebase StorageClient) *UserHandler {
	return &UserHandler{
		gcs:      gcs,
		firebase: firebase,
	}
}

func (h *UserHandler) GetStorageUsage(c *fiber.Ctx) error {
	uid, ok := c.Locals("uid").(string)
	if !ok || uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	var totalUsed int64

	// 1. Get GCS usage (Bot recordings)
	gcsUsed, err := h.gcs.GetTotalStorageUsed(c.Context(), uid)
	if err != nil {
		log.Printf("[UserHandler] Error getting GCS storage usage for %s: %v", uid, err)
	} else {
		totalUsed += gcsUsed
	}

	// 2. Get Firebase Storage usage (Offline recordings)
	// If they are the same bucket (per main.go logic), we might be double counting
	// IF we use the same prefix for both.
	// But GCS uses "recordings/{uid}/{botid}.mp4"
	// And Firebase uses "recordings/{uid}/{recordingId}.aac"
	// So listing "recordings/{uid}/" should cover both if they are in the same bucket.

	// If the buckets are different, we need to call both.
	if h.firebase != h.gcs {
		fbUsed, err := h.firebase.GetTotalStorageUsed(c.Context(), uid)
		if err != nil {
			log.Printf("[UserHandler] Error getting Firebase storage usage for %s: %v", uid, err)
		} else {
			totalUsed += fbUsed
		}
	}

	limit := int64(500 * 1024 * 1024) // 500MB

	return c.JSON(fiber.Map{
		"used_bytes":  totalUsed,
		"limit_bytes": limit,
	})
}
