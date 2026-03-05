# Executive Product Requirements Document: Notula Pro

## 1. Executive Summary

Notula Pro is an AI-powered meeting intelligence platform designed to eliminate manual documentation. By centralizing virtual (Recall.ai) and in-person (Gladia) meeting data into a single, searchable "Professional Memory," we empower teams to focus on decision-making rather than note-taking.

## 2. Strategic Objectives (Success Metrics)

To measure the impact of Notula Pro, the platform must achieve the following KPIs:

- **Transcription Latency**: 1 hour of meeting audio processed and synced within < 2 minutes.
- **Search Retrieval Time**: Full-text search across 100+ transcripts in < 500ms.
- **Cost Efficiency**: Long-term storage cost per user maintained at < $0.05/month via Hybrid GCS Archiving.
- **Reliability**: 99.9% success rate for Bot joining and recording stability.

## 3. Targeted User Personas & Stories

### Personas

- **The Project Lead**: Managing high-stakes team discussions and deliverables.
- **The Consultant**: Conducting on-site visits where mobile, offline capture is critical.
- **The Analyst**: Deep-diving into historical data to find recurring business insights.
- **The Business Owner**: Guarding sensitive client IP and ensuring organization-wide compliance.

### User Stories (The "Why")

- **The Project Lead**: *As a Project Lead, I want a bot to handle all meeting capture automatically, **so that I can lead the discussion without the distraction of manual note-taking.***
- **The Consultant**: *As a Consultant, I want to upload high-quality voice recordings from my phone after a client site visit, **so that I have a verified transcript of requirements without needing a laptop.***
- **The Analyst**: *As an Analyst, I want to perform a global search across months of transcripts, **so that I can identify recurring patterns and insights across different stakeholders instantly.***
- **The Business Owner**: *As a Business Owner, I want my meeting data isolated by user-ID and stored in private vaults, **so that I can guarantee my clients' intellectual property is never exposed to unauthorized parties.***

## 4. Functional Requirements

### 4.1. Unified Meeting Capture

- **Virtual (Recall.ai)**: Automated bots for Zoom, Google Meet, and MS Teams with real-time status tracking.
- **In-Person (Offline)**: High-speed asynchronous upload and transcription for local recordings.
- **Speaker Diarization**: Identification of different speakers to provide context in transcripts.

### 4.2. Post-Meeting Intelligence Workflow

- **Automated Ingest**: Webhooks trigger immediate transcription upon meeting termination.
- **Hybrid Storage Strategy**:
  - **Firestore**: Searchable text index & metadata.
  - **GCS (Vault)**: Permanent, private storage of high-resolution media.
  - **Recall.ai**: Transient capture engine with 7-day auto-deletion to minimize costs.

## 5. Security & Privacy (Executive Grade)

- **Data Sovereignty**: Ownership-validated access. UID checks mandatory for all media requests.
- **Signed URL Access**: Temporary, short-lived (15 min) Google Cloud Signed URLs for secure playback.
- **Architecture Isolation**: UID-specific directory structures in GCS (`recordings/<uid>/...`).
- **Encryption**: Data encrypted at rest (AES-256) and in transit (TLS 1.2+).

## 6. Scope & Roadmap

### In-Scope (V1)

- Core Fiber Backend with Firestore Integration.
- Recall.ai Bot Lifecycle & Webhook Automation. [IMPLEMENTED]
- Gladia V2 Offline Upload Integration.
- Hybrid GCS Archiving & Signed URL Retrieval.

### Out-of-Scope (V1)

- Real-time live transcription (Roadmap V2).
- Native Mobile iOS/Android Recording App (Roadmap V2).
- Automatic Calendar Sync (Outlook/Google) — (Scheduled for V1.1).

## 7. Technical Architecture Summary

- **Backend**: Go (Fiber) for high-concurrency performance.
- **Database**: Cloud Firestore for searchable metadata.
- **Storage**: Private Google Cloud Storage (GCS) Buckets.
- **Auth**: Firebase Identity Platform.

## 8. Compliance

- **GDPR Ready**: Support for "Right to be Forgotten" via automated GCS/Firestore deletion hooks.
- **Audit Logs**: Tracking of internal API calls for data access.
