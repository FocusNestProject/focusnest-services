package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

// Service handles Cloud Storage operations
type Service struct {
	client     *storage.Client
	bucketName string
}

// NewService creates a new storage service
func NewService(ctx context.Context, bucketName string) (*Service, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	return &Service{
		client:     client,
		bucketName: bucketName,
	}, nil
}

// UploadImage uploads an image to Cloud Storage and returns signed URLs
func (s *Service) UploadImage(ctx context.Context, userID string, imageData io.Reader, filename string) (*ImageUploadResult, error) {
	// Generate UUID for the activity
	activityID := uuid.New().String()

	// Get file extension
	ext := getFileExtension(filename)

	// Create object paths
	originalPath := fmt.Sprintf("original/%s/%s%s", userID, activityID, ext)
	overviewPath := fmt.Sprintf("overview/%s/%s.png", userID, activityID)

	// Upload original image
	originalURL, err := s.uploadObject(ctx, originalPath, imageData, "image/jpeg")
	if err != nil {
		return nil, fmt.Errorf("failed to upload original image: %w", err)
	}

	// Generate signed URL for overview (will be created later by overview service)
	overviewURL, err := s.generateSignedURL(ctx, overviewPath, 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("failed to generate overview signed URL: %w", err)
	}

	return &ImageUploadResult{
		ActivityID:   activityID,
		OriginalURL:  originalURL,
		OverviewURL:  overviewURL,
		OriginalPath: originalPath,
		OverviewPath: overviewPath,
	}, nil
}

// uploadObject uploads data to Cloud Storage and returns a signed URL
func (s *Service) uploadObject(ctx context.Context, objectPath string, data io.Reader, contentType string) (string, error) {
	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(objectPath)

	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType
	writer.CacheControl = "public, max-age=3600" // 1 hour cache

	_, err := io.Copy(writer, data)
	if err != nil {
		return "", fmt.Errorf("failed to write to storage: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	// Generate signed URL for the uploaded object
	signedURL, err := s.generateSignedURL(ctx, objectPath, 24*time.Hour)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return signedURL, nil
}

// generateSignedURL creates a signed URL for an object
func (s *Service) generateSignedURL(ctx context.Context, objectPath string, expiration time.Duration) (string, error) {
	bucket := s.client.Bucket(s.bucketName)

	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  "GET",
		Expires: time.Now().Add(expiration),
	}

	url, err := bucket.SignedURL(objectPath, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return url, nil
}

// ImageUploadResult contains the result of an image upload
type ImageUploadResult struct {
	ActivityID   string `json:"activity_id"`
	OriginalURL  string `json:"original_url"`
	OverviewURL  string `json:"overview_url"`
	OriginalPath string `json:"-"` // Internal path, not exposed in API
	OverviewPath string `json:"-"` // Internal path, not exposed in API
}

// getFileExtension extracts the file extension from filename
func getFileExtension(filename string) string {
	// Simple extension extraction
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i:]
		}
	}
	return ".jpg" // default fallback
}

// Close closes the storage client
func (s *Service) Close() error {
	return s.client.Close()
}
