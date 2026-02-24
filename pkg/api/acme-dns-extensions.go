package api

// acme-dns-extensions.go
//
// Adds a lightweight HTTP server on :9090 exposing:
//   GET /healthz  — liveness check (verifies the SQLite db file is accessible)
//   GET /metrics  — Prometheus metrics (acmedns_update_requests_total)
//
// startExtensionServer is called as a goroutine from AcmednsAPI.Start().

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var acmednsUpdateTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "acmedns_update_requests_total",
		Help: "Total /update API requests, labelled by status (success|failure)",
	},
	[]string{"status"},
)

func (a *AcmednsAPI) startExtensionServer() {
	if err := prometheus.Register(acmednsUpdateTotal); err != nil {
		a.Logger.Warnw("Failed to register Prometheus metric", "error", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", a.extensionHealthzHandler)
	srv := &http.Server{
		Addr:         ":9090",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	a.Logger.Infow("Extension server listening", "addr", ":9090")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		a.errChan <- err
	}
}

func (a *AcmednsAPI) extensionHealthzHandler(w http.ResponseWriter, r *http.Request) {
	dbPath := "/var/lib/acme-dns/acme-dns.db"
	w.Header().Set("Content-Type", "application/json")
	if _, err := os.Stat(dbPath); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  "database file not found",
		})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// RecordUpdate increments the update counter. Called from update.go.
func RecordUpdate(status string) {
	acmednsUpdateTotal.WithLabelValues(status).Inc()
}
