package recall

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"notulapro-backend/recall/events"
)

// setupMockServer creates a mock HTTP server that returns the given status code and response body.
// It returns the server and a Client configured to point to it.
func setupMockServer(statusCode int, responseBody interface{}) (*httptest.Server, *Client) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers are being set correctly
		if r.Header.Get("Authorization") != "Token test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(statusCode)
		if responseBody != nil {
			json.NewEncoder(w).Encode(responseBody)
		}
	}))

	client := &Client{
		apiKey:  "test-api-key",
		baseURL: server.URL,
		http:    server.Client(), // Use the server's clean HTTP client
	}

	return server, client
}

func TestClient_CreateBot(t *testing.T) {
	expectedBotID := "bot-123"
	mockResponse := events.BotResponse{
		ID: expectedBotID,
	}

	server, client := setupMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	joinAt := time.Now().Add(1 * time.Hour)
	bot, err := client.CreateBot("https://zoom.us/j/123", "TestBot", &joinAt)

	if err != nil {
		t.Fatalf("CreateBot returned error: %v", err)
	}

	if bot.ID != expectedBotID {
		t.Errorf("Expected bot ID %s, got %s", expectedBotID, bot.ID)
	}
}

func TestClient_GetBot(t *testing.T) {
	expectedBotID := "bot-456"
	mockResponse := events.BotResponse{
		ID: expectedBotID,
	}

	server, client := setupMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	bot, err := client.GetBot(expectedBotID)

	if err != nil {
		t.Fatalf("GetBot returned error: %v", err)
	}

	if bot.ID != expectedBotID {
		t.Errorf("Expected bot ID %s, got %s", expectedBotID, bot.ID)
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	// Test how the client handles a 400 Bad Request error
	server, client := setupMockServer(http.StatusBadRequest, map[string]string{"error": "invalid meeting URL"})
	defer server.Close()

	_, err := client.CreateBot("invalid-url", "", nil)

	if err == nil {
		t.Fatal("Expected an error for a 400 response, but got nil")
	}
}

func TestClient_LeaveBot(t *testing.T) {
	// LeaveBot just expects a 200 OK
	server, client := setupMockServer(http.StatusOK, nil)
	defer server.Close()

	err := client.LeaveBot("bot-789")

	if err != nil {
		t.Fatalf("LeaveBot returned error: %v", err)
	}
}
