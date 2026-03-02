# Product Requirements Document: Notula Pro

## 1. Executive Summary
Notula Pro is a meeting intelligence platform that automates documentation for virtual and in-person meetings. It leverages AI to provide transcription, summarization, and organized notes.

## 2. Problem Statement
Manual note-taking is inefficient and prone to error. Virtual meetings often lack a centralized, searchable record of discussions, while in-person meetings are even harder to document systematically.

## 3. Product Vision
To be the "memory" for every professional interaction, allowing users to focus on the conversation rather than the documentation.

## 4. Target Audience
- Project Managers
- Researchers and Journalists
- Enterprise Teams
- Freelancers

## 5. Key Features

### 5.1. Virtual Meeting Bot (Recall.ai)
- Automatically join Zoom, Google Meet, and MS Teams.
- Capture audio and video.
- Real-time bot status tracking (Joining, In-call, Recording).

### 5.2. In-Person Meeting Recording (Gladia Integration)
- **Offline Upload**: Support for uploading high-quality audio recordings post-meeting.
- **Asynchronous Transcription**: Fast processing (1 hour of audio in < 1 minute).
- **Metadata Support**: Title, tags, and speaker identification.

### 5.3. Meeting Intelligence
- Automated transcription via Gladia and Recall.ai.
- (Roadmap) LLM-powered summarization and action item extraction.
- (Roadmap) Speaker diarization.

## 6. Technical Architecture

### 6.1. Tech Stack
- **Backend**: Go (Fiber)
- **Database**: Firebase Firestore
- **Auth**: Firebase Auth
- **Orchestration**: Recall.ai
- **Transcription (Offline)**: Gladia API

### 6.2. In-Person Meeting Flow (Recommendation)
For maximum reliability, we recommend the **Local Record & Upload** approach for in-person meetings:
1. **App Level**: Record audio locally on the device (ensures no data loss on network drops).
2. **Backend Level**: Upload the completed file to the `/recording/offline` endpoint.
3. **Gladia Integration**: Backend forwards the audio to Gladia's Asynchronous API for high-speed transcription.

## 7. Compliance & Security
- Data isolation via Firebase UID.
- Secure API key management via environment variables.

## 8. Git Tracking
This document is tracked in the repository to ensure alignment across the development team.
