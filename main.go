package main

import (
	"context"
	"job-applications/internal/database"
	"job-applications/internal/server"
	"job-applications/internal/templates"
	"job-applications/internal/utils"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {

	// When running on dev, a .env file is needed for faster development
	// In test and prod, .env files are used to provide environments variable directly to the container
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	logLevel := slog.LevelDebug
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	slog.SetDefault(slog.New(handler))

	db, errConnect := database.Connect(context.Background(), utils.GetEnv("POSTGRES_DB_STRING", "invalid"))
	if errConnect != nil {
		slog.Error("Unable to connect to database", "error", errConnect)
		os.Exit(1)
	}
	renderer := templates.New("templates/*.html")

	srv := server.New(server.Config{
		Port:              utils.GetEnv("PORT", "9000"),
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 15 * time.Second,
	}, db, renderer)

	go func() {
		log.Printf("Server starting on port %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
