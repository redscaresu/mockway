package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/redscaresu/mockway/repository"
)

func (app *Application) CreateRDBInstance(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}

	if initEndpoints, ok := body["init_endpoints"]; ok {
		endpoints, err := repository.BuildRDBEndpointsFromInit(initEndpoints, body["engine"])
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid init_endpoints", "type": "invalid_argument"})
			return
		}
		if len(endpoints) > 0 {
			first, _ := endpoints[0].(map[string]any)
			if pnObj, ok := first["private_network"].(map[string]any); ok {
				pnID, _ := pnObj["id"].(string)
				exists, err := app.repo.Exists("private_networks", "id", pnID)
				if err != nil {
					writeDomainError(w, err)
					return
				}
				if !exists {
					writeJSON(w, http.StatusNotFound, map[string]any{"message": "referenced resource not found", "type": "not_found"})
					return
				}
			}
		}
		body["endpoints"] = endpoints
		delete(body, "init_endpoints")
	}

	out, err := app.repo.CreateRDBInstance(chi.URLParam(r, "region"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetRDBInstance(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetRDBInstance(chi.URLParam(r, "instance_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListRDBInstances(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListRDBInstances(chi.URLParam(r, "region"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "instances", items)
}

func (app *Application) UpdateRDBInstance(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateRDBInstance(chi.URLParam(r, "instance_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetRDBCertificate(w http.ResponseWriter, r *http.Request) {
	if _, err := app.repo.GetRDBInstance(chi.URLParam(r, "instance_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	// Return a fake certificate that the provider expects.
	writeJSON(w, http.StatusOK, map[string]any{
		"certificate": map[string]any{
			"content": "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALHMPMCJ+OebMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBm1v\nY2t3YTAeFw0yNDAyMjQwMDAwMDBaFw0zNDAyMjQwMDAwMDBaMBExDzANBgNVBAMM\nBm1vY2t3YTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o35FHQOGT7Pmb+oCaFHh\nOBAAPHlNmjNKHEl2hdNRMNwIDAQABMA0GCSqGSIb3DQEBCwUAA0EA\n-----END CERTIFICATE-----\n",
		},
	})
}

func (app *Application) DeleteRDBInstance(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteRDBInstance(chi.URLParam(r, "instance_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateRDBDatabase(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	name, _ := body["name"].(string)
	out, err := app.repo.CreateRDBDatabase(chi.URLParam(r, "instance_id"), name, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListRDBDatabases(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListRDBDatabases(chi.URLParam(r, "instance_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "databases", items)
}

func (app *Application) DeleteRDBDatabase(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteRDBDatabase(chi.URLParam(r, "instance_id"), chi.URLParam(r, "db_name")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateRDBUser(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	name, _ := body["name"].(string)
	out, err := app.repo.CreateRDBUser(chi.URLParam(r, "instance_id"), name, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListRDBUsers(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListRDBUsers(chi.URLParam(r, "instance_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "users", items)
}

func (app *Application) DeleteRDBUser(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteRDBUser(chi.URLParam(r, "instance_id"), chi.URLParam(r, "user_name")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) SetRDBACLs(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	rules, _ := body["rules"].([]any)
	if rules == nil {
		rules = []any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func (app *Application) ListRDBACLs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"rules": []any{}, "total_count": 0})
}

func (app *Application) DeleteRDBACLs(w http.ResponseWriter, _ *http.Request) {
	writeNoContent(w)
}

func (app *Application) SetRDBPrivileges(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	instanceID := chi.URLParam(r, "instance_id")
	privileges, _ := body["privileges"].([]any)
	if privileges == nil {
		privileges = []any{}
	}
	result, err := app.repo.SetRDBPrivileges(instanceID, privileges)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"privileges": result, "total_count": len(result)})
}

func (app *Application) ListRDBPrivileges(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instance_id")
	result, err := app.repo.ListRDBPrivileges(instanceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// Support filtering by database_name and user_name query params.
	if dbName := r.URL.Query().Get("database_name"); dbName != "" {
		filtered := make([]map[string]any, 0)
		for _, p := range result {
			if p["database_name"] == dbName {
				filtered = append(filtered, p)
			}
		}
		result = filtered
	}
	if userName := r.URL.Query().Get("user_name"); userName != "" {
		filtered := make([]map[string]any, 0)
		for _, p := range result {
			if p["user_name"] == userName {
				filtered = append(filtered, p)
			}
		}
		result = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"privileges": result, "total_count": len(result)})
}

func (app *Application) SetRDBSettings(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	settings, _ := body["settings"].([]any)
	if settings == nil {
		settings = []any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}
