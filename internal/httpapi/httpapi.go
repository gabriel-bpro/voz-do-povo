// Package httpapi is the Go API service: the only interface the web app uses
// to read the curated layer.
package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// server carries the API's dependencies (today: just the curated-layer DB).
type server struct {
	db *sql.DB
}

// New returns the API's HTTP handler rooted at the given database.
func New(db *sql.DB) http.Handler {
	s := &server{db: db}
	mux := http.NewServeMux()

	// Liveness only — no database touch.
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Database connectivity: reports the database's own clock.
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		var now time.Time
		if err := db.QueryRowContext(r.Context(), `SELECT now()`).Scan(&now); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"database": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"database":   "ok",
			"serverTime": now.UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("GET /api/politicians/search", s.handleSearch)
	mux.HandleFunc("GET /api/politicians/{id}", s.handleProfile)

	return mux
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
