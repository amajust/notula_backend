package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"notulapro-backend/recall/events"

	"github.com/gofiber/fiber/v2"
)

// ─── Mocks ───────────────────────────────────────────────────────────────────

type mockRecallClient struct {
	createBotFunc               func(meetingURL string, botName string, joinAt *time.Time) (*events.BotResponse, error)
	getBotFunc                  func(botID string) (*events.BotResponse, error)
	leaveBotFunc                func(botID string) error
	deleteBotFunc               func(botID string) error
	startAsyncTranscriptionFunc func(recordingID string) error
	getTranscriptFunc           func(botID string) ([]events.TranscriptElement, error)
	deleteMediaFunc             func(botID string) error
	sendChatMessageFunc         func(botID string, text string) error
}

func (m *mockRecallClient) CreateBot(meetingURL string, botName string, joinAt *time.Time) (*events.BotResponse, error) {
	if m.createBotFunc != nil {
		return m.createBotFunc(meetingURL, botName, joinAt)
	}
	return &events.BotResponse{ID: "mock-bot"}, nil
}
func (m *mockRecallClient) GetBot(botID string) (*events.BotResponse, error) {
	if m.getBotFunc != nil {
		return m.getBotFunc(botID)
	}
	return &events.BotResponse{ID: botID}, nil
}
func (m *mockRecallClient) LeaveBot(botID string) error {
	if m.leaveBotFunc != nil {
		return m.leaveBotFunc(botID)
	}
	return nil
}
func (m *mockRecallClient) DeleteBot(botID string) error {
	if m.deleteBotFunc != nil {
		return m.deleteBotFunc(botID)
	}
	return nil
}
func (m *mockRecallClient) StartAsyncTranscription(recordingID string) error {
	if m.startAsyncTranscriptionFunc != nil {
		return m.startAsyncTranscriptionFunc(recordingID)
	}
	return nil
}
func (m *mockRecallClient) GetTranscript(botID string) ([]events.TranscriptElement, error) {
	if m.getTranscriptFunc != nil {
		return m.getTranscriptFunc(botID)
	}
	return []events.TranscriptElement{}, nil
}
func (m *mockRecallClient) DeleteMedia(botID string) error {
	if m.deleteMediaFunc != nil {
		return m.deleteMediaFunc(botID)
	}
	return nil
}
func (m *mockRecallClient) SendChatMessage(botID string, text string) error {
	if m.sendChatMessageFunc != nil {
		return m.sendChatMessageFunc(botID, text)
	}
	return nil
}

type mockBotRepository struct {
	getActiveBotFunc              func(ctx context.Context, meetingURL string) (string, error)
	getScheduledBotFunc           func(ctx context.Context, meetingURL string) (string, error)
	getLatestBotFunc              func(ctx context.Context, meetingURL string) (map[string]interface{}, error)
	saveBotFunc                   func(ctx context.Context, bot map[string]interface{}) error
	getBotByIDFunc                func(ctx context.Context, botID string) (map[string]interface{}, error)
	updateBotStatusFunc           func(ctx context.Context, botID string, status string) error
	updateBotStatusAndSubCodeFunc func(ctx context.Context, botID string, status string, subCode string) error
	saveTranscriptFunc            func(ctx context.Context, botID string, transcript interface{}) error
	deleteBotLocallyFunc          func(ctx context.Context, botID string) error
}

func (m *mockBotRepository) GetActiveBotByMeetingURL(ctx context.Context, meetingURL string) (string, error) {
	if m.getActiveBotFunc != nil {
		return m.getActiveBotFunc(ctx, meetingURL)
	}
	return "", nil
}
func (m *mockBotRepository) GetScheduledBotByMeetingURL(ctx context.Context, meetingURL string) (string, error) {
	if m.getScheduledBotFunc != nil {
		return m.getScheduledBotFunc(ctx, meetingURL)
	}
	return "", nil
}
func (m *mockBotRepository) GetLatestBotByMeetingURL(ctx context.Context, meetingURL string) (map[string]interface{}, error) {
	if m.getLatestBotFunc != nil {
		return m.getLatestBotFunc(ctx, meetingURL)
	}
	return nil, nil
}
func (m *mockBotRepository) SaveBot(ctx context.Context, bot map[string]interface{}) error {
	if m.saveBotFunc != nil {
		return m.saveBotFunc(ctx, bot)
	}
	return nil
}
func (m *mockBotRepository) GetBotByID(ctx context.Context, botID string) (map[string]interface{}, error) {
	if m.getBotByIDFunc != nil {
		return m.getBotByIDFunc(ctx, botID)
	}
	return nil, nil
}
func (m *mockBotRepository) UpdateBotStatus(ctx context.Context, botID string, status string) error {
	return nil
}
func (m *mockBotRepository) UpdateBotStatusAndSubCode(ctx context.Context, botID string, status string, subCode string) error {
	return nil
}
func (m *mockBotRepository) SaveTranscript(ctx context.Context, botID string, transcript interface{}) error {
	if m.saveTranscriptFunc != nil {
		return m.saveTranscriptFunc(ctx, botID, transcript)
	}
	return nil
}
func (m *mockBotRepository) DeleteBotLocally(ctx context.Context, botID string) error {
	if m.deleteBotLocallyFunc != nil {
		return m.deleteBotLocallyFunc(ctx, botID)
	}
	return nil
}

type mockBotScheduler struct {
	scheduleBotFunc func(ctx context.Context, uid string, userName string, meetingURL string, joinAt time.Time) (string, error)
}

func (m *mockBotScheduler) ScheduleBot(ctx context.Context, uid string, userName string, meetingURL string, joinAt time.Time) (string, error) {
	if m.scheduleBotFunc != nil {
		return m.scheduleBotFunc(ctx, uid, userName, meetingURL, joinAt)
	}
	return "bot-scheduled", nil
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestBotHandler_SendBot_Success(t *testing.T) {
	app := fiber.New()

	mockRecall := &mockRecallClient{
		createBotFunc: func(meetingURL string, botName string, joinAt *time.Time) (*events.BotResponse, error) {
			return &events.BotResponse{ID: "bot-123"}, nil
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

	handler := NewBotHandler(mockRecall, mockRepo, nil, &mockBotScheduler{})

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
		getLatestBotFunc: func(ctx context.Context, meetingURL string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"id":     "existing-bot-id",
				"status": "joining",
			}, nil
		},
	}

	handler := NewBotHandler(&mockRecallClient{}, mockRepo, nil, &mockBotScheduler{})
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
		createBotFunc: func(meetingURL string, botName string, joinAt *time.Time) (*events.BotResponse, error) {
			return &events.BotResponse{ID: "bot-scheduled"}, nil
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

	handler := NewBotHandler(mockRecall, mockRepo, nil, &mockBotScheduler{})

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
	handler := NewBotHandler(nil, nil, nil, &mockBotScheduler{})

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
		getBotFunc: func(botID string) (*events.BotResponse, error) {
			return &events.BotResponse{ID: botID}, nil
		},
	}
	handler := NewBotHandler(mockRecall, nil, nil, &mockBotScheduler{})
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
	handler := NewBotHandler(mockRecall, nil, nil, &mockBotScheduler{})
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
	handler := NewBotHandler(mockRecall, nil, nil, &mockBotScheduler{})
	app.Post("/recording/:id/transcript", handler.StartTranscript)

	req := httptest.NewRequest(http.MethodPost, "/recording/rec-123/transcript", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}
