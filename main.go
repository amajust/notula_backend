package main

import (
	"context"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"

	"notulapro-backend/gladia"
	"notulapro-backend/handlers"
	"notulapro-backend/middleware"
	"notulapro-backend/recall"
	"notulapro-backend/repository"
)

func main() {
	// Load .env if present (ignored in production where env vars are injected)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	if os.Getenv("RECALL_API_KEY") == "" {
		log.Fatal("RECALL_API_KEY environment variable is required")
	}

	gladiaAPIKey := os.Getenv("GLADIA_API_KEY")
	if gladiaAPIKey == "" {
		log.Println("⚠️ GLADIA_API_KEY not set. Offline transcriptions will fail.")
	}
	gladiaClient := gladia.NewClient(gladiaAPIKey)

	// ─── Firebase Admin SDK ───────────────────────────────────────────────────────
	ctx := context.Background()

	var firebaseOpts []option.ClientOption
	if credFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); credFile != "" {
		firebaseOpts = append(firebaseOpts, option.WithCredentialsFile(credFile))
	}
	// If GOOGLE_APPLICATION_CREDENTIALS is not set, Application Default Credentials
	// will be used (works automatically on GCP / Cloud Run).

	fbApp, err := firebase.NewApp(ctx, nil, firebaseOpts...)
	if err != nil {
		log.Fatalf("error initializing Firebase app: %v", err)
	}

	authClient, err := fbApp.Auth(ctx)
	if err != nil {
		log.Fatalf("error initializing Firebase Auth client: %v", err)
	}

	firestoreClient, err := fbApp.Firestore(ctx)
	if err != nil {
		log.Fatalf("error initializing Firestore client: %v", err)
	}
	defer firestoreClient.Close()

	// ─── Fiber app ───────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName: "Notula Backend v1",
		// Increase limit for audio uploads (e.g. 50MB)
		BodyLimit: 50 * 1024 * 1024,
	})

	// ─── Global middleware ────────────────────────────────────────────────────────
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*", // Restrict in production to your app's origin
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// ─── Public routes ────────────────────────────────────────────────────────────
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "notulapro-backend",
		})
	})

	// ─── Protected routes (require valid Firebase ID token) ───────────────────────
	recallClient := recall.New()
	botRepo := repository.NewFirestoreBotRepository(firestoreClient)
	botHandler := handlers.NewBotHandler(recallClient, botRepo)

	recordingRepo := repository.NewFirestoreRecordingRepository(firestoreClient)
	recordingHandler := handlers.NewRecordingHandler(recordingRepo, gladiaClient)

	api := app.Group("/api/v1", middleware.FirebaseAuth(authClient))

	// Bot routes
	api.Post("/bot/send", botHandler.SendBot)
	api.Post("/bot/schedule", botHandler.ScheduleBot)
	api.Get("/bot/:id", botHandler.GetBotStatus)
	api.Post("/bot/:id/leave", botHandler.LeaveBot)

	// Recording / transcription routes
	api.Post("/recording/:id/transcript", botHandler.StartTranscript)
	api.Post("/recording/offline", recordingHandler.UploadOfflineRecording)

	// ─── Start ────────────────────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Notula backend starting on :%s", port)
	log.Fatal(app.Listen(":" + port))
}
