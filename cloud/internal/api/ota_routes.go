package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.temporal.io/sdk/client"

	"github.com/example/temporal-hack/cloud/internal/ota"
)

// OTADeps is wired by the control plane main into NewRouter.
type OTADeps struct {
	Temporal    client.Client
	RegistryURL string // e.g. http://localhost:14050 — used by GET /v1/ota/images
}

// AttachOTARoutes adds:
//
//	POST   /v1/ota/rollouts             start a rollout, return id
//	GET    /v1/ota/rollouts             list rollouts (paged)
//	GET    /v1/ota/rollouts/{id}        rollout summary
//	POST   /v1/ota/rollouts/{id}/abort  cancel an in-flight rollout
//	GET    /v1/ota/images               list available image refs from the registry
func (d Deps) AttachOTARoutes(mux *http.ServeMux, ot OTADeps) {
	mux.HandleFunc("POST /v1/ota/rollouts", d.startRollout(ot))
	mux.HandleFunc("GET /v1/ota/rollouts", d.listRollouts)
	mux.HandleFunc("GET /v1/ota/rollouts/{id}", d.getRollout)
	mux.HandleFunc("POST /v1/ota/rollouts/{id}/abort", d.abortRollout(ot))
	mux.HandleFunc("GET /v1/ota/images", d.listImages(ot))
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

// ImageRef is a deployable container image returned by GET /v1/ota/images.
// `Ref` is the full pull spec the agent shells to docker/podman.
type ImageRef struct {
	Repo string `json:"repo"`
	Tag  string `json:"tag"`
	Ref  string `json:"image_ref"`
}

func (d Deps) listImages(ot OTADeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := ot.RegistryURL
		if base == "" {
			http.Error(w, "registry not configured", http.StatusServiceUnavailable)
			return
		}
		base = trimSlash(base)

		// registryHost is what we put in the image_ref. The registry's
		// HTTP API is reached via base (e.g. http://localhost:14050)
		// but agents pull from `localhost:14050/<repo>:<tag>` — strip
		// the scheme.
		registryHost := strings.TrimPrefix(base, "http://")
		registryHost = strings.TrimPrefix(registryHost, "https://")

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var catalog struct {
			Repositories []string `json:"repositories"`
		}
		if err := getJSON(ctx, base+"/v2/_catalog", &catalog); err != nil {
			http.Error(w, "registry catalog: "+err.Error(), http.StatusBadGateway)
			return
		}

		out := make([]ImageRef, 0, len(catalog.Repositories))
		for _, repo := range catalog.Repositories {
			var tags struct {
				Tags []string `json:"tags"`
			}
			if err := getJSON(ctx, base+"/v2/"+repo+"/tags/list", &tags); err != nil {
				continue
			}
			for _, tag := range tags.Tags {
				out = append(out, ImageRef{
					Repo: repo,
					Tag:  tag,
					Ref:  registryHost + "/" + repo + ":" + tag,
				})
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return errors.New(resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var _ = errors.New
