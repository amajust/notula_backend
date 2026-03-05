package events

import (
	"context"
	"fmt"
	"log"
)

type RealtimeEventProcessor struct {
	Recall RecallClient
}

func NewRealtimeEventProcessor(r RecallClient) *RealtimeEventProcessor {
	return &RealtimeEventProcessor{Recall: r}
}

func (p *RealtimeEventProcessor) ProcessTranscript(ctx context.Context, event string, botID string, speakerName string, words []struct {
	Text string `json:"text"`
}) error {
	if event != "transcript.data" {
		return nil
	}

	var fullText string
	for _, w := range words {
		fullText += w.Text
	}

	if speakerName == "" {
		speakerName = "Participant"
	}

	if fullText != "" {
		chatMessage := fmt.Sprintf("[%s]: %s", speakerName, fullText)
		log.Printf("[RealtimeEvent] Sending chat message for bot %s", botID)
		return p.Recall.SendChatMessage(botID, chatMessage)
	}

	return nil
}
