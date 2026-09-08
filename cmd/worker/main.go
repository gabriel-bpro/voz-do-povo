// Worker service: runs the mirror's syncs on their per-source schedules.
// Sync failures are logs only — the app always renders the last good sync.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"voz-do-povo/internal/camara"
	"voz-do-povo/internal/store"
	"voz-do-povo/internal/sync"
)

// camaraLegislatures covers the 2023+ history: the 56th legislature ended
// 2023-01-31, the 57th runs 2023-02-01 → 2027-01-31.
var camaraLegislatures = []int{56, 57}

// historyStartYear is the mirror's historical depth for legislative data.
const historyStartYear = 2023

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

	client := camara.New()

	// One-shot history backfill: `worker bootstrap-camara` re-ingests the
	// CEAP bulk files for every year since historyStartYear, then exits.
	// The recurring jobs only cover the current year, so this runs once
	// (manually) after first deploy and after any history extension.
	if len(os.Args) > 1 && os.Args[1] == "bootstrap-camara" {
		for year := historyStartYear; year <= time.Now().Year(); year++ {
			if err := syncCamaraBulk(ctx, db, client, year); err != nil {
				log.Fatalf("bootstrap-camara %d: %v", year, err)
			}
		}
		log.Printf("bootstrap-camara done (%d→%d)", historyStartYear, time.Now().Year())
		return
	}

	jobs := []job{
		{"camara-registry", 24 * time.Hour, func(ctx context.Context) error {
			return syncCamaraRegistry(ctx, db, client)
		}},
		{"camara-expenses-delta", 24 * time.Hour, func(ctx context.Context) error {
			return syncCamaraDelta(ctx, db, client)
		}},
		{"camara-expenses-bulk", 7 * 24 * time.Hour, func(ctx context.Context) error {
			return syncCamaraBulk(ctx, db, client, time.Now().Year())
		}},
	}

	for _, j := range jobs {
		go j.runLoop(ctx)
	}

	log.Printf("worker started; %d sync jobs scheduled", len(jobs))
	<-ctx.Done()
	log.Printf("worker stopped")
}

// job is a scheduled sync. It runs once at startup, then every interval;
// panics and errors are logged, never fatal (logs-only failure policy).
type job struct {
	name  string
	every time.Duration
	run   func(ctx context.Context) error
}

func (j job) runLoop(ctx context.Context) {
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("sync %s panicked: %v", j.name, r)
				}
			}()
			start := time.Now()
			if err := j.run(ctx); err != nil {
				log.Printf("sync %s failed after %s: %v", j.name, time.Since(start).Round(time.Second), err)
			} else {
				log.Printf("sync %s ok in %s", j.name, time.Since(start).Round(time.Second))
			}
		}()

		select {
		case <-ctx.Done():
			return
		case <-time.After(j.every):
		}
	}
}

// syncCamaraRegistry refreshes the deputy registry: list both legislatures,
// fetch details (CPF, civil name) only for deputies we don't have a CPF for
// yet, then promote.
func syncCamaraRegistry(ctx context.Context, db *sql.DB, client *camara.Client) error {
	deputies, err := client.ListDeputies(ctx, camaraLegislatures...)
	if err != nil {
		return fmt.Errorf("list deputies: %w", err)
	}

	hasCPF, err := sync.CamaraDeputiesWithCPF(ctx, db)
	if err != nil {
		return err
	}
	for i := range deputies {
		if hasCPF[deputies[i].ID] {
			continue
		}
		if err := client.FillDetail(ctx, &deputies[i]); err != nil {
			// A missing detail must not sink the whole registry sync: log
			// and promote without CPF (linkage falls back to name+UF).
			log.Printf("sync camara-registry: detail for deputy %d failed: %v", deputies[i].ID, err)
		}
		time.Sleep(100 * time.Millisecond) // polite pacing; undocumented rate limit
	}

	if err := sync.SyncCamaraDeputies(ctx, db, deputies); err != nil {
		return fmt.Errorf("promote: %w", err)
	}
	log.Printf("sync camara-registry: %d deputies promoted", len(deputies))
	return nil
}

// syncCamaraDelta pulls the current and previous month of expenses per
// current-legislature deputy from the API (late postings land in the previous
// month) and promotes them.
func syncCamaraDelta(ctx context.Context, db *sql.DB, client *camara.Client) error {
	deputies, err := client.ListDeputies(ctx, 57)
	if err != nil {
		return fmt.Errorf("list deputies: %w", err)
	}

	now := time.Now()
	periods := [][2]int{{now.Year(), int(now.Month())}}
	prev := now.AddDate(0, -1, 0)
	periods = append(periods, [2]int{prev.Year(), int(prev.Month())})

	var all []camara.Expense
	for _, d := range deputies {
		for _, p := range periods {
			expenses, err := client.ListExpenses(ctx, d.ID, p[0], p[1])
			if err != nil {
				log.Printf("sync camara-expenses-delta: deputy %d %d-%02d failed: %v", d.ID, p[0], p[1], err)
				continue
			}
			all = append(all, expenses...)
		}
		time.Sleep(100 * time.Millisecond)
	}

	stats, err := sync.SyncCamaraExpenses(ctx, db, sync.SourceCamaraCEAPDelta, 0, all)
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}
	log.Printf("sync camara-expenses-delta: %+v", stats)
	return nil
}

// syncCamaraBulk re-ingests the CEAP bulk file for a year as ground-truth
// reconciliation.
func syncCamaraBulk(ctx context.Context, db *sql.DB, client *camara.Client, year int) error {
	var all []camara.Expense
	err := client.FetchBulkExpenses(ctx, year, func(e camara.Expense) error {
		all = append(all, e)
		return nil
	})
	if err != nil {
		return fmt.Errorf("fetch bulk %d: %w", year, err)
	}

	stats, err := sync.SyncCamaraExpenses(ctx, db,
		sync.SourceCamaraCEAPBulk+fmt.Sprint(year), year, all)
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}
	log.Printf("sync camara-expenses-bulk %d: %+v", year, stats)
	return nil
}
