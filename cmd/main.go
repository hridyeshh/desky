package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"desky/internal/db"
	"desky/internal/handlers"
)

func main() {
	// Load .env for local dev; ignore error if the file is absent (e.g. on Railway).
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	database, err := db.Connect(databaseURL)
	if err != nil {
		log.Fatalf("database connect: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	h := handlers.New(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /config", h.GetConfig)
	mux.HandleFunc("PUT /config", h.UpdateConfig)
	mux.HandleFunc("GET /events", h.Events)
	mux.HandleFunc("GET /widget/clock", h.Clock)
	mux.HandleFunc("GET /widget/weather", h.Weather)
	mux.HandleFunc("GET /widget/spotify", h.Spotify)

	// Power standby
	mux.HandleFunc("POST /api/power", h.SetPower)

	// Countdown timer
	mux.HandleFunc("POST /api/timer", h.SetTimer)

	// Heartbeat / connectivity
	mux.HandleFunc("POST /api/heartbeat", h.Heartbeat)
	mux.HandleFunc("GET /api/status", h.Status)

	// GIF upload (Cloudinary)
	mux.HandleFunc("POST /api/upload-gif", h.UploadGIF)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, handlers.CORS(handlers.Logging(mux))); err != nil {
		log.Fatalf("server: %v", err)
	}
}
