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

	gcsBucket := os.Getenv("GCS_BUCKET")
	if gcsBucket == "" {
		gcsBucket = "notula-recordings"
	}
	gcsClient, err := storage.NewGCSClient(ctx, gcsBucket)
	if err != nil {
		log.Printf("Warning: GCS storage not initialized: %v", err)
	}

	// Initialize Firebase Storage client
	firebaseBucket := os.Getenv("FIREBASE_STORAGE_BUCKET")
	if firebaseBucket == "" {
		firebaseBucket = "notulapro.firebasestorage.app"
	}
	firebaseStorageClient, err := storage.NewFirebaseStorageClient(ctx, firebaseBucket)
	if err != nil {
		log.Printf("Warning: Firebase Storage not initialized: %v", err)
	}

	// ─── Handlers ───
	botHandler := handlers.NewBotHandler(recallClient, botRepo, gcsClient)
	recordingHandler := handlers.NewRecordingHandler(recordingRepo, gladiaClient, firebaseStorageClient)
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

	api := app.Group("/api/v1", middleware.FirebaseAuth(authClient))

	// Webhooks (usually unauthenticated or secret-based, but grouping under /api for now)
	app.Post("/api/v1/webhook/recall", webhookHandler.RecallWebhook)
	app.Post("/api/v1/webhook/gladia", webhookHandler.GladiaWebhook)

	// Bot routes
	api.Post("/bot/send", botHandler.SendBot)
	api.Post("/bot/schedule", botHandler.ScheduleBot)
	api.Get("/bot/:id", botHandler.GetBotStatus)
	api.Post("/bot/:id/leave", botHandler.LeaveBot)

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
