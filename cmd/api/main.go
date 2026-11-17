// main wires the layers together and runs the HTTP server with graceful shutdown.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"golang_crud/auth"
	"golang_crud/config"
	"golang_crud/database"
	"golang_crud/handlers"
	"golang_crud/middleware"
	"golang_crud/repository"
	"golang_crud/routes"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	gin.SetMode(cfg.GinMode)

	// JSON logs in production (easy for log tools to parse), readable text in development.
	if cfg.GinMode == gin.ReleaseMode {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	}

	// 1. Database (+ migrations)
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if cfg.RunMigrations {
		if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
			return err
		}
		log.Println("migrations: up to date")
	}

	// 2. Build layers from the bottom up: repositories -> auth -> handlers -> router
	bookRepo := repository.NewBookRepository(db)
	userRepo := repository.NewUserRepository(db)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTokenTTL)

	r := routes.New(
		routes.Handlers{
			Books:  handlers.NewBookHandler(bookRepo),
			Health: handlers.NewHealthHandler(db),
			Auth:   handlers.NewAuthHandler(userRepo, tokens),
		},
		tokens,
		middleware.RateLimit(10, 5), // register/login: 10 requests/minute per IP, bursts of 5
	)

	// 3. HTTP server with timeouts (never use the default server in production)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on http://localhost:%s", cfg.Port)
		errCh <- srv.ListenAndServe()
	}()

	// 4. Wait for Ctrl+C / SIGTERM (Docker stop), then finish in-flight requests.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("shutting down...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
