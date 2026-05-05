package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"go.temporal.io/sdk/client"

	"github.com/example/temporal-hack/cloud/internal/ota"
)

// OTADeps is wired by the control plane main into NewRouter.
type OTADeps struct {
	Temporal client.Client
}

// AttachOTARoutes adds:
//
//	POST   /v1/ota/rollouts             start a rollout, return id
//	GET    /v1/ota/rollouts             list rollouts (paged)
//	GET    /v1/ota/rollouts/{id}        rollout summary
//	POST   /v1/ota/rollouts/{id}/abort  cancel an in-flight rollout
func (d Deps) AttachOTARoutes(mux *http.ServeMux, ot OTADeps) {
	mux.HandleFunc("POST /v1/ota/rollouts", d.startRollout(ot))
	mux.HandleFunc("GET /v1/ota/rollouts", d.listRollouts)
	mux.HandleFunc("GET /v1/ota/rollouts/{id}", d.getRollout)
	mux.HandleFunc("POST /v1/ota/rollouts/{id}/abort", d.abortRollout(ot))
}

func (d Deps) startRollout(ot OTADeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var spec ota.RolloutSpec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if spec.ImageRef == "" {
			http.Error(w, "image_ref required", http.StatusBadRequest)
			return
		}

		id := "rollout-" + randHex(8)
		opts := client.StartWorkflowOptions{
			ID:                                       id,
			TaskQueue:                                ota.TaskQueue,
			WorkflowExecutionTimeout:                 24 * time.Hour,
			WorkflowExecutionErrorWhenAlreadyStarted: true,
		}
		_, err := ot.Temporal.ExecuteWorkflow(r.Context(), opts, "OTARollout", spec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"rollout_id": id})
	}
}

func (d Deps) listRollouts(w http.ResponseWriter, r *http.Request) {
	rows, err := d.TSDB.ListRollouts(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (d Deps) getRollout(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	row, err := d.TSDB.GetRollout(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (d Deps) abortRollout(ot OTADeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		err := ot.Temporal.CancelWorkflow(r.Context(), id, "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var _ = errors.New
