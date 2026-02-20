package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *Application) CreateCluster(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateCluster(chi.URLParam(r, "region"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetCluster(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetCluster(chi.URLParam(r, "cluster_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListClusters(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListClusters(chi.URLParam(r, "region"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "clusters", items)
}

func (app *Application) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteCluster(chi.URLParam(r, "cluster_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreatePool(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreatePool(chi.URLParam(r, "region"), chi.URLParam(r, "cluster_id"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetPool(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetPool(chi.URLParam(r, "pool_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListPools(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListPoolsByCluster(chi.URLParam(r, "cluster_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "pools", items)
}

func (app *Application) DeletePool(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeletePool(chi.URLParam(r, "pool_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}
