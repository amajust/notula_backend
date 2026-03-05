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
		} `json:"media_shortcuts"`
	} `json:"recordings"`
}

type TranscriptElement struct {
	Speaker string  `json:"speaker"`
	Text    string  `json:"text"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
}
