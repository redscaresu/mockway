package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *Application) CreateLB(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateLB(chi.URLParam(r, "zone"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetLB(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetLB(chi.URLParam(r, "lb_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListLBs(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListLBs(chi.URLParam(r, "zone"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "lbs", items)
}

func (app *Application) DeleteLB(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteLB(chi.URLParam(r, "lb_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateFrontend(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateFrontend(body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetFrontend(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetFrontend(chi.URLParam(r, "frontend_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListFrontends(w http.ResponseWriter, _ *http.Request) {
	items, err := app.repo.ListFrontends()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "frontends", items)
}

func (app *Application) DeleteFrontend(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteFrontend(chi.URLParam(r, "frontend_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateBackend(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateBackend(body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetBackend(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetBackend(chi.URLParam(r, "backend_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListBackends(w http.ResponseWriter, _ *http.Request) {
	items, err := app.repo.ListBackends()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "backends", items)
}

func (app *Application) DeleteBackend(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteBackend(chi.URLParam(r, "backend_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) AttachLBPrivateNetwork(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	pnID, _ := body["private_network_id"].(string)
	out, err := app.repo.AttachLBPrivateNetwork(chi.URLParam(r, "lb_id"), pnID)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListLBPrivateNetworks(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListLBPrivateNetworks(chi.URLParam(r, "lb_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "private_networks", items)
}

func (app *Application) DeleteLBPrivateNetwork(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteLBPrivateNetwork(chi.URLParam(r, "lb_id"), chi.URLParam(r, "pn_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}
