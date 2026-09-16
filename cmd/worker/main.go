// Worker service: one-shot run of the mirror's syncs, fired daily by a
// Railway cron schedule. Sync failures are logs only for the app — it
// always renders the last good sync — but a failed sync exits non-zero so
// the cron run shows as failed.
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
	"voz-do-povo/internal/senado"
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
	senadoClient := senado.New()

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

	// Same backfill for the Senado: registry first (CEAPS links via
	// codSenador), then every CEAPS year dataset since historyStartYear.
	if len(os.Args) > 1 && os.Args[1] == "bootstrap-senado" {
		if err := syncSenadoRegistry(ctx, db, senadoClient); err != nil {
			log.Fatalf("bootstrap-senado registry: %v", err)
		}
		for year := historyStartYear; year <= time.Now().Year(); year++ {
			if err := syncSenadoExpensesYear(ctx, db, senadoClient, year); err != nil {
				log.Fatalf("bootstrap-senado %d: %v", year, err)
			}
		}
		log.Printf("bootstrap-senado done (%d→%d)", historyStartYear, time.Now().Year())
		return
	}

	// CEAP cadence: the bulk year files are updated daily and are the only
	// working CEAP source (the API's despesas endpoint returns empty for
	// every deputy — verified 2026-09-08), so the current-year bulk re-ingest
	// runs daily and doubles as the delta. CEAPS likewise: the year dataset
	// is updated daily upstream. Railway fires this process on a daily cron
	// schedule; each run executes every sync once and exits.
	jobs := []job{
		{"camara-registry", func(ctx context.Context) error {
			return syncCamaraRegistry(ctx, db, client)
		}},
		{"camara-expenses-bulk", func(ctx context.Context) error {
			return syncCamaraBulk(ctx, db, client, time.Now().Year())
		}},
		// Registry before expenses: CEAPS rows link via codSenador against
		// the registry-built source-id map.
		{"senado-registry", func(ctx context.Context) error {
			return syncSenadoRegistry(ctx, db, senadoClient)
		}},
		{"senado-ceaps", func(ctx context.Context) error {
			return syncSenadoExpensesYear(ctx, db, senadoClient, time.Now().Year())
		}},
	}

	failed := false
	for _, j := range jobs {
		if err := j.runOnce(ctx); err != nil {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

// job is a named sync. Errors and panics are logged (the app always renders
// the last good sync), but a failed job makes the run exit non-zero so the
// failure is visible in the cron run status.
type job struct {
	name string
	run  func(ctx context.Context) error
}

func (j job) runOnce(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("sync %s panicked: %v", j.name, r)
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	start := time.Now()
	if err := j.run(ctx); err != nil {
		log.Printf("sync %s failed after %s: %v", j.name, time.Since(start).Round(time.Second), err)
		return err
	}
	log.Printf("sync %s ok in %s", j.name, time.Since(start).Round(time.Second))
	return nil
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

// syncCamaraBulk re-ingests the CEAP bulk file for a year: freshness (the
// file is updated daily) and ground-truth reconciliation in one pass.
func syncCamaraBulk(ctx context.Context, db *sql.DB, client *camara.Client, year int) error {
	all, err := client.FetchBulkExpenses(ctx, year)
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

// syncSenadoRegistry refreshes the senator registry (senators in exercise).
// The Senado publishes no CPF, so cross-house linkage falls to the name+UF
// review queue inside the promote step.
func syncSenadoRegistry(ctx context.Context, db *sql.DB, client *senado.Client) error {
	senators, err := client.ListSenators(ctx)
	if err != nil {
		return fmt.Errorf("list senators: %w", err)
	}
	if err := sync.SyncSenadoSenators(ctx, db, senators); err != nil {
		return fmt.Errorf("promote: %w", err)
	}
	log.Printf("sync senado-registry: %d senators promoted", len(senators))
	return nil
}

// syncSenadoExpensesYear re-ingests the CEAPS year dataset: freshness (the
// dataset is updated daily upstream) and ground-truth reconciliation in one
// pass. CEAPS rows carry no party sigla, so the registry's sigla at sync
// time stands in.
func syncSenadoExpensesYear(ctx context.Context, db *sql.DB, client *senado.Client, year int) error {
	senators, err := client.ListSenators(ctx)
	if err != nil {
		return fmt.Errorf("list senators for party map: %w", err)
	}
	partyBySenator := make(map[int64]string, len(senators))
	for _, s := range senators {
		partyBySenator[s.Code] = s.PartySigla
	}

	all, err := client.FetchExpenses(ctx, year)
	if err != nil {
		return fmt.Errorf("fetch ceaps %d: %w", year, err)
	}

	stats, err := sync.SyncSenadoExpenses(ctx, db,
		sync.SourceSenadoCEAPSYear+fmt.Sprint(year), year, all, partyBySenator)
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}
	log.Printf("sync senado-ceaps %d: %+v", year, stats)
	return nil
}
