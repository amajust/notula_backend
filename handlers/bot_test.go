package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"notulapro-backend/recall"

	"github.com/gofiber/fiber/v2"
)

// ─── Mocks ───────────────────────────────────────────────────────────────────

type mockRecallClient struct {
	createBotFunc               func(meetingURL string, joinAt *time.Time) (*recall.BotResponse, error)
	getBotFunc                  func(botID string) (*recall.BotResponse, error)
	leaveBotFunc                func(botID string) error
	startAsyncTranscriptionFunc func(recordingID string) error
}

func (m *mockRecallClient) CreateBot(meetingURL string, joinAt *time.Time) (*recall.BotResponse, error) {
	return m.createBotFunc(meetingURL, joinAt)
}
func (m *mockRecallClient) GetBot(botID string) (*recall.BotResponse, error) {
	return m.getBotFunc(botID)
}
func (m *mockRecallClient) LeaveBot(botID string) error {
	return m.leaveBotFunc(botID)
}
func (m *mockRecallClient) StartAsyncTranscription(recordingID string) error {
	return m.startAsyncTranscriptionFunc(recordingID)
}

type mockBotRepository struct {
	getActiveBotFunc    func(ctx context.Context, meetingURL string) (string, error)
	getScheduledBotFunc func(ctx context.Context, meetingURL string) (string, error)
	saveBotFunc         func(ctx context.Context, bot map[string]interface{}) error
}

func (m *mockBotRepository) GetActiveBotByMeetingURL(ctx context.Context, meetingURL string) (string, error) {
	return m.getActiveBotFunc(ctx, meetingURL)
}
func (m *mockBotRepository) GetScheduledBotByMeetingURL(ctx context.Context, meetingURL string) (string, error) {
	return m.getScheduledBotFunc(ctx, meetingURL)
}
func (m *mockBotRepository) SaveBot(ctx context.Context, bot map[string]interface{}) error {
	return m.saveBotFunc(ctx, bot)
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestBotHandler_SendBot_Success(t *testing.T) {
	app := fiber.New()

	mockRecall := &mockRecallClient{
		createBotFunc: func(meetingURL string, joinAt *time.Time) (*recall.BotResponse, error) {
			return &recall.BotResponse{ID: "bot-123"}, nil
		},
	}

	mockRepo := &mockBotRepository{
		getActiveBotFunc: func(ctx context.Context, meetingURL string) (string, error) {
			return "", nil // No active bot
		},
		saveBotFunc: func(ctx context.Context, bot map[string]interface{}) error {
			if bot["uid"] != "user-123" {
				t.Errorf("Expected UID user-123, got %v", bot["uid"])
			}
			return nil
		},
	}

	handler := NewBotHandler(mockRecall, mockRepo)

	app.Post("/bot/send", func(c *fiber.Ctx) error {
		c.Locals("uid", "user-123")
		return handler.SendBot(c)
	})

	body := sendBotBody{MeetingURL: "https://zoom.us/j/123"}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/bot/send", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req)

	if resp.StatusCode != fiber.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}
}

func TestBotHandler_SendBot_Conflict(t *testing.T) {
	app := fiber.New()

	mockRepo := &mockBotRepository{
		getActiveBotFunc: func(ctx context.Context, meetingURL string) (string, error) {
			return "existing-bot-id", nil
		},
	}

	handler := NewBotHandler(nil, mockRepo)
	app.Post("/bot/send", handler.SendBot)

	body := sendBotBody{MeetingURL: "https://zoom.us/j/123"}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/bot/send", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req)

	if resp.StatusCode != fiber.StatusConflict {
		t.Errorf("Expected 409 for conflict, got %d", resp.StatusCode)
	}
}

func TestBotHandler_ScheduleBot_Success(t *testing.T) {
	app := fiber.New()

	mockRecall := &mockRecallClient{
		createBotFunc: func(meetingURL string, joinAt *time.Time) (*recall.BotResponse, error) {
			return &recall.BotResponse{ID: "bot-scheduled"}, nil
		},
	}

	mockRepo := &mockBotRepository{
		getScheduledBotFunc: func(ctx context.Context, meetingURL string) (string, error) {
			return "", nil
		},
		saveBotFunc: func(ctx context.Context, bot map[string]interface{}) error {
			return nil
		},
	}

	handler := NewBotHandler(mockRecall, mockRepo)

	app.Post("/bot/schedule", func(c *fiber.Ctx) error {
		c.Locals("uid", "user-123")
		return handler.ScheduleBot(c)
	})

	joinAt := time.Now().Add(15 * time.Minute)
	body := scheduleBotBody{
		MeetingURL: "https://zoom.us/j/123",
		JoinAt:     joinAt,
	}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/bot/schedule", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req)

	if resp.StatusCode != fiber.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}
}

func TestBotHandler_ScheduleBot_TooSoon(t *testing.T) {
	app := fiber.New()
	handler := NewBotHandler(nil, nil)

	app.Post("/bot/schedule", handler.ScheduleBot)

	joinAt := time.Now().Add(5 * time.Minute)
	body := scheduleBotBody{
		MeetingURL: "https://zoom.us/j/123",
		JoinAt:     joinAt,
	}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/bot/schedule", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req)

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 for scheduling < 10 mins away, got %d", resp.StatusCode)
	}
}

func TestBotHandler_GetBotStatus(t *testing.T) {
	app := fiber.New()
	mockRecall := &mockRecallClient{
		getBotFunc: func(botID string) (*recall.BotResponse, error) {
			return &recall.BotResponse{ID: botID}, nil
		},
	}
	handler := NewBotHandler(mockRecall, nil)
	app.Get("/bot/:id", handler.GetBotStatus)

	req := httptest.NewRequest(http.MethodGet, "/bot/bot-123", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestBotHandler_LeaveBot(t *testing.T) {
	app := fiber.New()
	mockRecall := &mockRecallClient{
		leaveBotFunc: func(botID string) error {
			return nil
		},
	}
	handler := NewBotHandler(mockRecall, nil)
	app.Post("/bot/:id/leave", handler.LeaveBot)

	req := httptest.NewRequest(http.MethodPost, "/bot/bot-123/leave", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestBotHandler_StartTranscript(t *testing.T) {
	app := fiber.New()
	mockRecall := &mockRecallClient{
		startAsyncTranscriptionFunc: func(recordingID string) error {
			return nil
		},
	}
	handler := NewBotHandler(mockRecall, nil)
	app.Post("/recording/:id/transcript", handler.StartTranscript)

	req := httptest.NewRequest(http.MethodPost, "/recording/rec-123/transcript", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}
