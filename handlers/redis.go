package handlers

import (
	"encoding/base64"
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

	// The SDK's File struct has Content []byte, so JSON expects base64-encoded data.
	const pem = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALHMPMCJ+OebMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBm1v\nY2t3YTAeFw0yNDAyMjQwMDAwMDBaFw0zNDAyMjQwMDAwMDBaMBExDzANBgNVBAMM\nBm1vY2t3YTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o35FHQOGT7Pmb+oCaFHh\nOBAAPHlNmjNKHEl2hdNRMNwIDAQABMA0GCSqGSIb3DQEBCwUAA0EA\n-----END CERTIFICATE-----\n"
	writeJSON(w, http.StatusOK, map[string]any{
		"content": base64.StdEncoding.EncodeToString([]byte(pem)),
	})
}

func (app *Application) MigrateRedisCluster(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	// Apply the migrate patch (node_type, version) so that provider reads see
	// the updated values after a migration call.
	patch := map[string]any{}
	for _, field := range []string{"node_type", "version"} {
		if v, ok := body[field]; ok {
			patch[field] = v
		}
	}
	patch["status"] = "ready"
	out, err := app.repo.UpdateRedisCluster(chi.URLParam(r, "cluster_id"), patch)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteRedisCluster(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "cluster_id")
	out, err := app.repo.GetRedisCluster(clusterID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if err := app.repo.DeleteRedisCluster(clusterID); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) SetRedisACLRules(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	rules, _ := body["acl_rules"].([]any)
	if rules == nil {
		rules = []any{}
	}
	out, err := app.repo.SetRedisACLRules(chi.URLParam(r, "cluster_id"), rules)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) SetRedisEndpoints(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	endpoints, _ := body["endpoints"].([]any)
	if endpoints == nil {
		endpoints = []any{}
	}
	out, err := app.repo.SetRedisEndpoints(chi.URLParam(r, "cluster_id"), endpoints)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) SetRedisClusterSettings(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	// The Scaleway Redis API defines settings as an array of ClusterSetting objects.
	settings, _ := body["settings"].([]any)
	if settings == nil {
		settings = []any{}
	}
	out, err := app.repo.SetRedisClusterSettings(chi.URLParam(r, "cluster_id"), settings)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteRedisEndpoint(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.DeleteRedisEndpoint(chi.URLParam(r, "zone"), chi.URLParam(r, "endpoint_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListRedisClusterVersions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"versions": []any{
			map[string]any{"version": "7.0.12", "end_of_life_at": nil, "available_settings": []any{}},
			map[string]any{"version": "6.2.14", "end_of_life_at": nil, "available_settings": []any{}},
		},
		"total_count": 2,
	})
}

func (app *Application) ListRedisNodeTypes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"node_types": []any{
			map[string]any{"name": "RED1-MICRO", "stock_status": "available", "memory": float64(1000000000), "vcpus": float64(1)},
			map[string]any{"name": "RED1-SMALL", "stock_status": "available", "memory": float64(2000000000), "vcpus": float64(2)},
			map[string]any{"name": "RED1-MEDIUM", "stock_status": "available", "memory": float64(4000000000), "vcpus": float64(4)},
		},
		"total_count": 3,
	})
}
