// Package testharness implements the repo's two agreed test seams:
//
//   - Seam 1 (HTTP API): boot the real API handler over HTTP against a real,
//     migrated Postgres; tests drive requests and assert responses.
//   - Seam 2 (sync): feed recorded source fixtures into a sync entry point
//     against a real Postgres; tests assert raw + curated table state.
//
// Both seams share TestDB, which spins up a throwaway Postgres in Docker.
package testharness

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"voz-do-povo/internal/httpapi"
	"voz-do-povo/internal/store"
)

// TestDB starts a throwaway Postgres container, waits for readiness, runs all
// migrations, and returns an open connection. The container is removed when
// the test finishes.
func TestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	name := fmt.Sprintf("vdp-test-%d", time.Now().UnixNano())
	run := exec.CommandContext(ctx, "docker", "run", "-d", "--rm",
		"--name", name,
		"-e", "POSTGRES_PASSWORD=test",
		"-p", "127.0.0.1:0:5432",
		"postgres:16-alpine")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("start test postgres: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		exec.Command("docker", "rm", "-f", name).Run()
	})

	portOut, err := exec.CommandContext(ctx, "docker", "port", name, "5432").Output()
	if err != nil {
		t.Fatalf("get test postgres port: %v", err)
	}
	// Output looks like "127.0.0.1:49153" (one line per bind addr); take the
	// port after the last colon of the first line.
	first := strings.Split(strings.TrimSpace(string(portOut)), "\n")[0]
	port := first[strings.LastIndex(first, ":")+1:]
	dsn := fmt.Sprintf("postgres://postgres:test@127.0.0.1:%s/postgres?sslmode=disable", port)

	var db *sql.DB
	deadline := time.Now().Add(60 * time.Second)
	for {
		db, err = store.Open(ctx, dsn)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("test postgres never became ready: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Cleanup(func() { db.Close() })

	if err := store.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate test postgres: %v", err)
	}
	return db
}

// API is seam 1: the real API handler served over HTTP, backed by a fresh
// test database. Seed the curated layer via the returned server URL's DB, or
// use the db directly through TestDB alongside this.
func API(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(httpapi.New(TestDB(t)))
	t.Cleanup(srv.Close)
	return srv
}

// Fixture reads a file from the calling test's testdata directory.
func Fixture(t *testing.T, name string) []byte {
	t.Helper()
	_, caller, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("resolve caller for fixture path")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(caller), "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}
