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
