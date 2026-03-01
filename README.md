# Notula Pro Backend

The backend service for Notula Pro, built with Go and the [Fiber](https://gofiber.io/) web framework. This service handles interactions with [Recall.ai](https://recall.ai/) for bots and recordings, as well as Firebase Auth and Firestore for data persistence and authentication.

## Prerequisites

- **Go 1.22+**
- **Docker** and **Docker Compose** (optional, for deployment)
- **Firebase Account** (Service Account JSON key required)
- **Recall.ai Account** (API Key required)

## Environment Variables

Copy `.env.example` to `.env` and configure your environment:

```bash
cp .env.example .env
```

| Variable                         | Description                                | Default                    |
| -------------------------------- | ------------------------------------------ | -------------------------- |
| `RECALL_API_KEY`                 | Your Recall.ai API Key                     | *Required*                 |
| `RECALL_REGION`                  | Recall.ai region (e.g., `us-west-2`)       | `us-west-2`                |
| `PORT`                           | The port the server runs on                | `8080`                     |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to Firebase service account key JSON  | `./serviceAccountKey.json` |

## Getting Started

### Local Development

1. Install Go modules:

   ```bash
   go mod download
   ```

2. Place your `serviceAccountKey.json` from Firebase in the root directory.
3. Run the application:

   ```bash
   go run main.go
   ```

### Running with Docker

You can spin up the backend using Docker Compose:

```bash
docker-compose up --build
```

*Note: Make sure to map your `serviceAccountKey.json` as a volume if needed, by uncommenting the appropriate lines in `docker-compose.yml`.*

## Architecture & API

### Public Endpoints

- `GET /health` : Health check endpoint.

### Protected API routes (`/api/v1`)

All API endpoints require a valid Firebase ID Token passed in the `Authorization` header as a Bearer token.

**Recall.ai Bot Management:**

- `POST /bot/send`: Send a recording bot to an active meeting.
- `POST /bot/schedule`: Schedule a bot for a future meeting.
- `GET /bot/:id`: Fetch the status of a specific bot.
- `POST /bot/:id/leave`: Force a bot to leave an active meeting.

**Recordings & Transcriptions:**

- `POST /recording/:id/transcript`: Request or start processing the transcript for a recorded meeting.
- `POST /recording/offline`: Upload a locally captured offline recording into Firestore.

## Project Structure

- `handlers/` - Request controllers dealing with bots and recordings.
- `middleware/` - Fiber middleware for Firebase Auth, CORS, Logger, etc.
- `recall/` - The interface/client implementation for communicating with Recall.ai.
- `main.go` - Application entry point, router setup, and Firebase admin initialization.

## License

MIT License
