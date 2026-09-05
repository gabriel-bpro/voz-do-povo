// Worker service: runs the mirror's syncs on their per-source schedules.
// Real syncs register here starting with ticket 12; for now the worker proves
// the deploy unit: it migrates and heartbeats.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"voz-do-povo/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}

	db, err := store.Open(ctx, dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := store.Migrate(ctx, db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	log.Printf("worker started; no syncs registered yet (ticket 12)")
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("worker stopped")
			return
		case <-ticker.C:
			log.Printf("worker heartbeat")
		}
	}
}
