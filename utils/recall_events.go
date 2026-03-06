package utils

// GetFriendlyProcessingStatus maps a Recall.ai `status` or webhook event (e.g., "joining_call")
// into a user-friendly UI string.
func GetFriendlyProcessingStatus(status string) string {
	switch status {
	case "joining_call", "joining":
		return "Bot is connecting..."
	case "in_waiting_room":
		return "Bot waiting in lobby..."
	case "in_call_not_recording":
		return "Bot joined (not recording)"
	case "recording_permission_allowed":
		return "Recording permission allowed"
	case "recording_permission_denied":
		return "Permission to Record Denied"
	case "in_call_recording":
		return "Bot is Recording..."
	case "processing":
		return "Media Processing..."
	case "breakout_room_opened":
		return "Breakout rooms available"
	case "breakout_room_entered":
		return "Bot entering breakout room"
	case "breakout_room_left":
		return "Bot leaving breakout room"
	case "breakout_room_closed":
		return "Breakout rooms closed"
	case "recording_done":
		return "Media Ready (Fetching Transcript...)"
	case "recorded", "transcript.done":
		return "Recorded (Transcript Ready)"
	case "done":
		return "Media Ready (Finalizing...)"
	case "transcript.processing":
		return "Generating Transcript..."
	case "transcript.failed", "failed", "call_ended", "fatal":
		return "Failed / Ended prematurely"
	case "paused":
		return "Recording Paused"
	case "deleted":
		return "Recording Deleted"
	case "completed", "archived":
		return "Completed"
	}

	// Fallbacks
	if len(status) > 8 && status[:8] == "breakout" {
		return "Bot in Breakout Room"
	}

	return "Processing bot..."
}
