package utils

import "strings"

// GetFriendlyRecallMessage maps a Recall.ai `sub_code` to a user-friendly error or status message.
func GetFriendlyRecallMessage(subCode string) string {
	if subCode == "" {
		return "Failed / Ended prematurely"
	}

	// Call Ended Sub Codes
	switch subCode {
	case "call_ended_by_host":
		return "Meeting Ended by Host"
	case "call_ended_by_platform_idle":
		return "Meeting Ended: Platform idle"
	case "call_ended_by_platform_max_length":
		return "Meeting Ended: Maximum length reached"
	case "call_ended_by_platform_waiting_room_timeout":
		return "Failed: Platform waiting room timeout exceeded"
	case "timeout_exceeded_waiting_room":
		return "Failed: Waiting room timeout exceeded"
	case "timeout_exceeded_noone_joined":
		return "Failed: Left because no one joined"
	case "timeout_exceeded_everyone_left":
		return "Meeting Ended: Everyone else left"
	case "timeout_exceeded_silence_detected":
		return "Meeting Ended: Continuous silence detected"
	case "timeout_exceeded_only_bots_detected_using_participant_names",
		"timeout_exceeded_only_bots_detected_using_participant_events":
		return "Meeting Ended: Only bots detected in call"
	case "timeout_exceeded_in_call_not_recording":
		return "Failed: Bot never started recording (timeout)"
	case "timeout_exceeded_in_call_recording":
		return "Meeting Ended: Recording timeout exceeded"
	case "timeout_exceeded_recording_permission_denied":
		return "Failed: Recording permission denied timeout"
	case "timeout_exceeded_max_duration":
		return "Failed: Maximum bot duration exceeded"
	case "bot_kicked_from_call":
		return "Failed: Bot kicked from call by host"
	case "bot_kicked_from_waiting_room":
		return "Failed: Bot kicked from waiting room by host"
	case "bot_received_leave_call":
		return "Meeting Ended: Bot received leave command"
	case "meeting_not_started":
		return "Failed: Meeting had not started yet"

	// Generic Fatal Sub Codes
	case "bot_errored":
		return "Failed: Bot encountered an unexpected error"
	case "meeting_not_accessible":
		return "Blocked: Meeting not accessible (check host settings)"
	case "meeting_not_found":
		return "Failed: No meeting found at this link"
	case "meeting_requires_registration":
		return "Blocked: Meeting requires Registration to join"
	case "meeting_requires_sign_in":
		return "Blocked: Meeting requires Sign-In to join"
	case "meeting_link_expired":
		return "Failed: Meeting link has expired"
	case "meeting_link_invalid":
		return "Failed: Meeting link is invalid"
	case "meeting_password_incorrect":
		return "Failed: Meeting password incorrect"
	case "meeting_locked":
		return "Blocked: Meeting is locked by host"
	case "meeting_full":
		return "Failed: Meeting is full"
	case "meeting_ended":
		return "Failed: Meeting already ended"
	case "failed_to_launch_in_time":
		return "Failed: Bot failed to launch in time"

	// Zoom
	case "zoom_registration_required":
		return "Blocked: Meeting requires Registration"
	case "zoom_captcha_required":
		return "Blocked: Meeting requires Captcha"
	case "zoom_email_required":
		return "Blocked: Meeting requires Email to join"
	case "zoom_email_blocked_by_admin":
		return "Blocked: Domain disallowed by Zoom Admin"
	case "zoom_meeting_not_accessible":
		return "Blocked: Region block or host disabled joining"
	case "zoom_account_blocked":
		return "Blocked: Bot account previously removed by host"
	case "zoom_invalid_join_token", "zoom_token_expired":
		return "Failed: Invalid or expired Zoom token"
	case "zoom_error_multiple_device_join":
		return "Failed: Appears bot joined multiple times"
	case "zoom_another_meeting_in_progress":
		return "Failed: Host has another meeting in progress"

	// Google Meet
	case "google_meet_internal_error":
		return "Failed: Google Meet internal error"
	case "google_meet_sign_in_failed":
		return "Failed: Google Meet sign-in failed"
	case "google_meet_sign_in_captcha_failed":
		return "Blocked: Google Meet sign-in required captcha"
	case "google_meet_bot_blocked":
		return "Blocked: Bot was disallowed from joining Google Meet"
	case "google_meet_sso_sign_in_failed":
		return "Failed: Google Meet SSO sign-in failed"
	case "google_meet_sign_in_missing_login_credentials":
		return "Failed: Google Meet login credentials missing"
	case "google_meet_sign_in_missing_recovery_credentials":
		return "Failed: Google Meet recovery credentials missing"
	case "google_meet_sso_sign_in_missing_login_credentials":
		return "Failed: Google Meet SSO login credentials missing"
	case "google_meet_sso_sign_in_missing_totp_secret":
		return "Failed: Google Meet SSO TOTP secret missing"
	case "google_meet_video_error":
		return "Failed: Google Meet video error"
	case "google_meet_meeting_room_not_ready":
		return "Failed: Google Meet room not ready"
	case "google_meet_login_not_available":
		return "Failed: No Google Meet logins available"
	case "google_meet_permission_denied_breakout":
		return "Failed: Denied entry to Google Meet breakout room"
	case "google_meet_knocking_disabled":
		return "Blocked: Google Meet knocking disabled by host"
	case "google_meet_watermark_kicked":
		return "Blocked: Google Meet watermark enabled mid-call"
	case "google_meet_organisation_restricted":
		return "Blocked: Google Meet restricted to host organization"

	// Teams
	case "microsoft_teams_sign_in_credentials_missing":
		return "Failed: Teams sign-in credentials missing"
	case "microsoft_teams_call_dropped":
		return "Failed: Teams call dropped unexpectedly"
	case "microsoft_teams_sign_in_failed":
		return "Failed: Teams sign-in failed"
	case "microsoft_teams_internal_error":
		return "Failed: Microsoft Teams server error"
	case "microsoft_teams_captcha_error":
		return "Blocked: Teams captcha required"
	case "microsoft_teams_bot_not_invited":
		return "Blocked: Bot not invited to Teams meeting"
	case "microsoft_teams_breakout_room_unsupported":
		return "Failed: Teams breakout rooms unsupported"
	case "microsoft_teams_townhall_meeting_not_supported":
		return "Failed: Teams townhall meetings unsupported"
	case "microsoft_teams_event_not_started_for_external":
		return "Blocked: Event not started for external participants"
	case "microsoft_teams_2fa_required":
		return "Blocked: Teams 2FA required for bot account"
	case "microsoft_teams_captions_failure":
		return "Failed: Host disabled Teams meeting captions"

	// Webex
	case "webex_pin_required", "webex_password_required":
		return "Blocked: Webex requires PIN or Password"
	case "webex_service_app_unauthorized":
		return "Failed: Webex app unauthorized"
	}

	// Fuzzy Fallback Matches
	if strings.Contains(subCode, "google_meet_sign_in") {
		return "Failed: Google Meet sign-in error"
	}
	if strings.Contains(subCode, "microsoft_teams_sign_in") {
		return "Failed: Teams sign-in error"
	}

	// Absolute Fallback: Format the slug string intelligently
	parts := strings.Split(subCode, ".")
	slug := subCode
	if len(parts) > 0 {
		slug = parts[len(parts)-1]
	}

	friendly := strings.TrimSpace(strings.ReplaceAll(slug, "_", " "))

	if len(friendly) > 0 {
		return "Failed: " + strings.ToUpper(friendly[:1]) + friendly[1:]
	}

	return "Failed: " + subCode
}
