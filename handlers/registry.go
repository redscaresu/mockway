package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *Application) CreateRegistryNamespace(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateRegistryNamespace(chi.URLParam(r, "region"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetRegistryNamespace(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetRegistryNamespace(chi.URLParam(r, "namespace_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListRegistryNamespaces(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListRegistryNamespaces(chi.URLParam(r, "region"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "namespaces", items)
}

func (app *Application) UpdateRegistryNamespace(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateRegistryNamespace(chi.URLParam(r, "namespace_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteRegistryNamespace(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteRegistryNamespace(chi.URLParam(r, "namespace_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}
