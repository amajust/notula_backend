package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"strings"

	"cloud.google.com/go/storage"
)

type GCSClient struct {
	client *storage.Client
	bucket string
}

func NewGCSClient(ctx context.Context, bucketName string) (*GCSClient, error) {
	// Strip gs:// prefix if present
	bucketName = strings.TrimPrefix(bucketName, "gs://")

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &GCSClient{
		client: client,
		bucket: bucketName,
	}, nil
}

// UploadFromURL downloads a file from a URL and uploads it to GCS.
func (g *GCSClient) UploadFromURL(ctx context.Context, url, objectName string) (string, error) {
	// 1. Download the file
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download from URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download from URL: status %d", resp.StatusCode)
	}

	// 2. Prepare GCS writer
	sw := g.client.Bucket(g.bucket).Object(objectName).NewWriter(ctx)
	if _, err := io.Copy(sw, resp.Body); err != nil {
		return "", fmt.Errorf("failed to copy data to GCS: %v", err)
	}

	if err := sw.Close(); err != nil {
		return "", fmt.Errorf("failed to close GCS writer: %v", err)
	}

	return fmt.Sprintf("gs://%s/%s", g.bucket, objectName), nil
}

// GenerateSignedURL creates a short-lived URL for the private GCS object.
func (g *GCSClient) GenerateSignedURL(objectName string, expires time.Duration) (string, error) {
	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  "GET",
		Expires: time.Now().Add(expires),
	}

	// Note: In production, the service account must have "Service Account Token Creator" role.
	return g.client.Bucket(g.bucket).SignedURL(objectName, opts)
}

// GetPath returns the relative path in the bucket.
func (g *GCSClient) GetPath(uid, botID string) string {
	return fmt.Sprintf("recordings/%s/%s.mp4", uid, botID)
}
