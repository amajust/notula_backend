package events

type BotResponse struct {
	ID            string                   `json:"id"`
	StatusChanges []map[string]interface{} `json:"status_changes"`
	Recordings    []struct {
		ID              string `json:"id"`
		DurationSeconds int    `json:"duration_seconds"`
		MediaShortcuts  struct {
			VideoMixed struct {
				Data struct {
					DownloadURL string `json:"download_url"`
				} `json:"data"`
			} `json:"video_mixed"`
			Transcript struct {
				ID     string `json:"id"`
				Status struct {
					Code string `json:"code"`
				} `json:"status"`
			} `json:"transcript"`
			ParticipantEvents struct {
				ID   string `json:"id"`
				Data struct {
					ParticipantsDownloadURL string `json:"participants_download_url"`
				} `json:"data"`
			} `json:"participant_events"`
		} `json:"media_shortcuts"`
	} `json:"recordings"`
	Transcripts []struct {
		ID     string `json:"id"`
		Status struct {
			Code string `json:"code"`
		} `json:"status"`
	} `json:"transcripts"`
}

type TranscriptElement struct {
	Speaker   string  `json:"speaker"`
	Text      string  `json:"text"`
	Start     float64 `json:"start"`
	End       float64 `json:"end"`
	Timestamp string  `json:"timestamp"`
}

type Calendar struct {
	ID                 string                 `json:"id"`
	Platform           string                 `json:"platform"`
	Status             string                 `json:"status"`
	AutomaticRecording map[string]interface{} `json:"automatic_recording"`
}

type CalendarOauthURL struct {
	OauthURL string `json:"oauth_url"`
}
