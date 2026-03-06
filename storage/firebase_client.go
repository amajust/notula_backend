package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// FirebaseStorageClient handles file uploads to Firebase Storage
type FirebaseStorageClient struct {
	client *storage.Client
	bucket string
}

// NewFirebaseStorageClient creates a new Firebase Storage client
func NewFirebaseStorageClient(ctx context.Context, bucketName string) (*FirebaseStorageClient, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	return &FirebaseStorageClient{
		client: client,
		bucket: bucketName,
	}, nil
}

// UploadFile uploads a multipart file to Firebase Storage
// Path format: recordings/{uid}/{recordingId}.aac
func (c *FirebaseStorageClient) UploadFile(file *multipart.FileHeader, storagePath string) error {
	ctx := context.Background()

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Get bucket handle
	bucket := c.client.Bucket(c.bucket)

	// Create object handle
	obj := bucket.Object(storagePath)

	// Create writer
	wc := obj.NewWriter(ctx)
	wc.ContentType = "audio/aac"
	wc.Metadata = map[string]string{
		"uploadedAt": time.Now().Format(time.RFC3339),
	}

	// Copy file content to storage
	if _, err := io.Copy(wc, src); err != nil {
		wc.Close()
		return fmt.Errorf("failed to write file to storage: %w", err)
	}

	// Close writer (this finalizes the upload)
	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close storage writer: %w", err)
	}

	log.Printf("[FirebaseStorage] Uploaded file to: %s", storagePath)
	return nil
}

// GenerateSignedURL generates a publicly accessible URL with Firebase Storage token
// For Firebase Storage, we use the public URL with download token instead of signed URLs
func (c *FirebaseStorageClient) GenerateSignedURL(storagePath string, expiration time.Duration) (string, error) {
	// For Firebase Storage, generate a public URL with token
	// Format: https://firebasestorage.googleapis.com/v0/b/{bucket}/o/{path}?alt=media

	// URL encode the storage path
	encodedPath := storagePath
	// Replace slashes with %2F for URL encoding
	encodedPath = ""
	for _, char := range storagePath {
		if char == '/' {
			encodedPath += "%2F"
		} else {
			encodedPath += string(char)
		}
	}

	// Generate public URL
	url := fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/%s?alt=media",
		c.bucket, encodedPath)

	log.Printf("[FirebaseStorage] Generated public URL for: %s", storagePath)
	return url, nil
}

// DeleteFile deletes a file from Firebase Storage
func (c *FirebaseStorageClient) DeleteFile(storagePath string) error {
	ctx := context.Background()

	bucket := c.client.Bucket(c.bucket)
	obj := bucket.Object(storagePath)

	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	log.Printf("[FirebaseStorage] Deleted file: %s", storagePath)
	return nil
}

// Close closes the storage client
func (c *FirebaseStorageClient) Close() error {
	return c.client.Close()
}

// GetTotalStorageUsed calculates the total size of all objects under recordings/{uid}/
func (c *FirebaseStorageClient) GetTotalStorageUsed(ctx context.Context, uid string) (int64, error) {
	prefix := fmt.Sprintf("recordings/%s/", uid)
	it := c.client.Bucket(c.bucket).Objects(ctx, &storage.Query{
		Prefix: prefix,
	})

	var totalSize int64
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("failed to list objects: %v", err)
		}
		totalSize += attrs.Size
	}

	return totalSize, nil
}
