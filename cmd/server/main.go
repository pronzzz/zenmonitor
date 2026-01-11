package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pronzzz/zenmonitor/internal/config"
	"github.com/pronzzz/zenmonitor/internal/monitor"
	"github.com/pronzzz/zenmonitor/internal/notifier"
	"github.com/pronzzz/zenmonitor/internal/store"
	"github.com/pronzzz/zenmonitor/internal/web"
	// Import sqlite driver implicitly
	_ "modernc.org/sqlite"
)

func main() {
	log.Println("Starting ZenMonitor...")

	// 1. Load Config
	// In Docker, we might map /app/config/monitors.yaml or just monitors.yaml in cwd
	// Let's try explicit first, then cwd
	configPath := "monitors.yaml"
	if os.Getenv("CONFIG_PATH") != "" {
		configPath = os.Getenv("CONFIG_PATH")
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Loaded %d monitors from %s", len(cfg.Monitors), configPath)

	// 2. Init Store
	// Ensure data directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Printf("Warning: failed to create data dir: %v", err)
	}
	dbPath := "data/zen.db"
	st, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database at %s: %v", dbPath, err)
	}
	defer st.Close()

	// Prune old data on startup
	go func() {
		if err := st.PruneOldData(cfg.Global.HistoryDays); err != nil {
			log.Printf("Failed to prune old data: %v", err)
		}
	}()

	// 3. Init Notifier
	notif := notifier.NewService(cfg.Notifications)

	// 4. Init & Start Monitor Engine
	engine := monitor.NewEngine(cfg, st, notif)
	engine.Start()
	log.Println("Monitoring engine started.")
	defer engine.Stop()

	// 5. Setup Web Server
	handler := web.NewHandler(st, cfg)

	port := "8080"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	
	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	go func() {
		log.Printf("Web server listening on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// 6. Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
	// Engine stops via defer
	// Store closes via defer
	// Server shutdown could be explicit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("ZenMonitor stopped.")
}
