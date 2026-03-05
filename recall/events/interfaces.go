package events

import (
	"context"
)

type RecallClient interface {
	GetBot(botID string) (*BotResponse, error)
	GetTranscript(recordingID string) ([]TranscriptElement, error)
	SendChatMessage(botID string, text string) error
	DeleteMedia(botID string) error
}

type BotRepository interface {
	UpdateBotStatus(ctx context.Context, botID string, status string) error
	UpdateBotStatusAndSubCode(ctx context.Context, botID string, status string, subCode string) error
	SaveTranscript(ctx context.Context, botID string, transcript interface{}) error
	GetBotByID(ctx context.Context, botID string) (map[string]interface{}, error)
	SaveBot(ctx context.Context, data map[string]interface{}) error
}

type RecordingRepository interface {
	UpdateRecording(ctx context.Context, recordingID string, updates any) error
}

type GCSClient interface {
	GetPath(uid string, botID string) string
	UploadFromURL(ctx context.Context, url string, objectName string) (string, error)
}
