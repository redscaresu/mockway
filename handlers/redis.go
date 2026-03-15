package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *Application) CreateRedisCluster(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateRedisCluster(chi.URLParam(r, "zone"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetRedisCluster(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetRedisCluster(chi.URLParam(r, "cluster_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListRedisClusters(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListRedisClusters(chi.URLParam(r, "zone"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "clusters", items)
}

func (app *Application) UpdateRedisCluster(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateRedisCluster(chi.URLParam(r, "cluster_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetRedisCertificate(w http.ResponseWriter, r *http.Request) {
	if _, err := app.repo.GetRedisCluster(chi.URLParam(r, "cluster_id")); err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"certificate": map[string]any{
			"content": "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALHMPMCJ+OebMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBm1v\nY2t3YTAeFw0yNDAyMjQwMDAwMDBaFw0zNDAyMjQwMDAwMDBaMBExDzANBgNVBAMM\nBm1vY2t3YTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o35FHQOGT7Pmb+oCaFHh\nOBAAPHlNmjNKHEl2hdNRMNwIDAQABMA0GCSqGSIb3DQEBCwUAA0EA\n-----END CERTIFICATE-----\n",
		},
	})
}

func (app *Application) DeleteRedisCluster(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteRedisCluster(chi.URLParam(r, "cluster_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}
