package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *Application) CreateVPC(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateVPC(chi.URLParam(r, "region"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetVPC(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetVPC(chi.URLParam(r, "vpc_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListVPCs(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListVPCs(chi.URLParam(r, "region"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "vpcs", items)
}

func (app *Application) DeleteVPC(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteVPC(chi.URLParam(r, "vpc_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreatePrivateNetwork(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreatePrivateNetwork(chi.URLParam(r, "region"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetPrivateNetwork(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetPrivateNetwork(chi.URLParam(r, "pn_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListPrivateNetworks(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListPrivateNetworks(chi.URLParam(r, "region"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "private_networks", items)
}

func (app *Application) DeletePrivateNetwork(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeletePrivateNetwork(chi.URLParam(r, "pn_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}
