package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"voz-do-povo/internal/testharness"
)

// Seam 1: drive the API over HTTP against a real (migrated) Postgres.
func TestHealthReportsDatabaseTime(t *testing.T) {
	srv := testharness.API(t)

	res, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}

	var body struct {
		Database   string `json:"database"`
		ServerTime string `json:"serverTime"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Database != "ok" {
		t.Errorf("database = %q, want %q", body.Database, "ok")
	}
	serverTime, err := time.Parse(time.RFC3339, body.ServerTime)
	if err != nil {
		t.Fatalf("serverTime %q does not parse as RFC3339: %v", body.ServerTime, err)
	}
	if delta := time.Since(serverTime); delta < -time.Minute || delta > time.Minute {
		t.Errorf("serverTime %v is %v from test clock, want within a minute", serverTime, delta)
	}
}
