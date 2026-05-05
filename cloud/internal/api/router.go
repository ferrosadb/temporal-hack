package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/example/temporal-hack/cloud/internal/store"
)

type Deps struct {
	TSDB   *store.Timescale
	Logger *slog.Logger
}

// NewRouter wires the v1 control-plane HTTP API. OTA routes are
// optional and are attached only when the temporal client is wired
// (the controlplane main passes nil OTADeps when running without
// Temporal access; in that mode the OTA endpoints 404).
//
//	GET    /healthz                          liveness
//	GET    /v1/robots                        list robots seen via heartbeats
//	GET    /v1/robots/{id}/telemetry         recent samples
//	POST   /v1/ota/rollouts                  start a rollout
//	GET    /v1/ota/rollouts                  list rollouts
//	GET    /v1/ota/rollouts/{id}             rollout summary
//	POST   /v1/ota/rollouts/{id}/abort       cancel a rollout
func NewRouter(d Deps, ota *OTADeps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /v1/robots", d.listRobots)
	mux.HandleFunc("GET /v1/robots/{id}/telemetry", d.recentTelemetry)
	if ota != nil {
		d.AttachOTARoutes(mux, *ota)
	}
	return mux
}

func (d Deps) listRobots(w http.ResponseWriter, r *http.Request) {
	rows, err := d.TSDB.ListRobots(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (d Deps) recentTelemetry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stream := r.URL.Query().Get("stream")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	since := time.Now().Add(-1 * time.Hour)
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	rows, err := d.TSDB.RecentTelemetry(r.Context(), id, stream, since, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
