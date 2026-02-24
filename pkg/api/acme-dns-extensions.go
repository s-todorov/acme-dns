package api

// acme-dns-extensions.go
//
// Adds a lightweight HTTP server on :9090 exposing:
//   GET /healthz  — liveness check (verifies the SQLite db file is accessible)
//   GET /metrics  — Prometheus metrics (acmedns_update_requests_total)
//
// startExtensionServer is called as a goroutine from AcmednsAPI.Start().

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
	addr := ":" + a.Config.API.MetricsPort
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	a.Logger.Infow("Extension server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		a.errChan <- err
	}
}

func (a *AcmednsAPI) extensionHealthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.DB.GetBackend().PingContext(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// recordUpdate increments the update counter. Called from update.go.
func recordUpdate(status string) {
	acmednsUpdateTotal.WithLabelValues(status).Inc()
}
