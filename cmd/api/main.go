// main wires the layers together and runs the HTTP server with graceful shutdown.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"golang_crud/config"
	"golang_crud/database"
	"golang_crud/handlers"
	"golang_crud/repository"
	"golang_crud/routes"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.Load()
	gin.SetMode(cfg.GinMode)

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

	// 2. Build layers from the bottom up: repository -> handlers -> router
	bookRepo := repository.NewBookRepository(db)
	r := router.New(
		handlers.NewBookHandler(bookRepo),
		handlers.NewHealthHandler(db),
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
