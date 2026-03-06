package main

import (
	"context"
	"log"
	"os"
	"time"

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
	"notulapro-backend/recall/events"
	recallhandlers "notulapro-backend/recall/handlers"
	"notulapro-backend/repository"
	"notulapro-backend/storage"
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
	// ─── Clients & Repositories ───
	recallClient := recall.New()
	botRepo := repository.NewFirestoreBotRepository(firestoreClient)
	recordingRepo := repository.NewFirestoreRecordingRepository(firestoreClient)

	firebaseBucket := os.Getenv("FIREBASE_STORAGE_BUCKET")
	if firebaseBucket == "" {
		firebaseBucket = "notulapro.firebasestorage.app"
	}

	gcsBucket := os.Getenv("GCS_BUCKET")
	if gcsBucket == "" {
		// Fallback to firebase bucket as a more sensible default than a hardcoded string
		gcsBucket = firebaseBucket
	}
	log.Printf("[Storage] Using GCS bucket: %s", gcsBucket)

	gcsClient, err := storage.NewGCSClient(ctx, gcsBucket)
	if err != nil {
		log.Printf("Warning: GCS storage not initialized: %v", err)
	}

	log.Printf("[Storage] Using Firebase Storage bucket: %s", firebaseBucket)
	firebaseStorageClient, err := storage.NewFirebaseStorageClient(ctx, firebaseBucket)
	if err != nil {
		log.Printf("Warning: Firebase Storage not initialized: %v", err)
	}

	// ─── Handlers ───
	botHandler := handlers.NewBotHandler(recallClient, botRepo, gcsClient)
	recordingHandler := handlers.NewRecordingHandler(recordingRepo, gladiaClient, firebaseStorageClient)
	userHandler := handlers.NewUserHandler(gcsClient, firebaseStorageClient)

	// ─── Recall Webhook System (Granular) ───
	botEventProc := events.NewBotEventProcessor(botRepo)
	recEventProc := events.NewRecordingEventProcessor(botRepo, recallClient)
	transcriptEventProc := events.NewTranscriptEventProcessor(botRepo, recallClient, gcsClient)
	realtimeEventProc := events.NewRealtimeEventProcessor(recallClient)

	recallBotHandler := recallhandlers.NewBotHandler(botEventProc)
	recallRecHandler := recallhandlers.NewRecordingHandler(recEventProc)
	recallTranscriptHandler := recallhandlers.NewTranscriptHandler(transcriptEventProc)
	recallRealtimeHandler := recallhandlers.NewRealtimeHandler(realtimeEventProc)

	// Use old webhookHandler only for Gladia if needed
	webhookHandler := handlers.NewWebhookHandler(recallClient, botRepo, recordingRepo, gcsClient, gladiaClient)

	// ─── Recovery: Check for stuck recordings on startup ───
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	go func() {
		time.Sleep(5 * time.Second) // Wait for server to be ready
		if err := handlers.RecoverStuckRecordings(ctx, recordingRepo, gladiaClient, firebaseStorageClient, baseURL); err != nil {
			log.Printf("[Recovery] Error during startup recovery: %v", err)
		}
	}()

	// Webhooks (must be defined BEFORE the /api/v1 auth group to bypass middleware)
	webhook := app.Group("/api/v1/webhook/recall", middleware.RecallWebhookAuth())
	webhook.Post("/bot", recallBotHandler.Handle)
	webhook.Post("/recording", recallRecHandler.Handle)
	webhook.Post("/transcript", recallTranscriptHandler.Handle)
	webhook.Post("/realtime", recallRealtimeHandler.Handle)
	app.Post("/api/v1/webhook/gladia", webhookHandler.GladiaWebhook)

	api := app.Group("/api/v1", middleware.FirebaseAuth(authClient))

	// Bot routes
	api.Post("/bot/send", botHandler.SendBot)
	api.Post("/bot/schedule", botHandler.ScheduleBot)
	api.Get("/bot/:id", botHandler.GetBotStatus)
	api.Get("/bot/:id/transcript", botHandler.GetBotTranscript)
	api.Post("/bot/:id/leave", botHandler.LeaveBot)
	api.Delete("/bot/:id", botHandler.DeleteBot)

	// User routes
	api.Get("/user/storage", userHandler.GetStorageUsage)

	// Recording routes
	api.Get("/recording/:id/url", botHandler.GetRecordingURL)
	api.Post("/recording/offline", recordingHandler.UploadOfflineRecording)

	// ─── Start ────────────────────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Notula backend starting on :%s", port)
	log.Fatal(app.Listen(":" + port))
}
