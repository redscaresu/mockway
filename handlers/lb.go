package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *Application) CreateLBIP(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateLBIP(chi.URLParam(r, "zone"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetLBIP(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetLBIP(chi.URLParam(r, "ip_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListLBIPs(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListLBIPs(chi.URLParam(r, "zone"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "ips", items)
}

func (app *Application) DeleteLBIP(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteLBIP(chi.URLParam(r, "ip_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

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

func (app *Application) UpdateLB(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateLB(chi.URLParam(r, "lb_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
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
	if lbID := chi.URLParam(r, "lb_id"); lbID != "" {
		body["lb_id"] = lbID
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

func (app *Application) ListFrontends(w http.ResponseWriter, r *http.Request) {
	lbID := chi.URLParam(r, "lb_id")
	var items []map[string]any
	var err error
	if lbID != "" {
		items, err = app.repo.ListFrontendsByLB(lbID)
	} else {
		items, err = app.repo.ListFrontends()
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "frontends", items)
}

func (app *Application) ListFrontendACLs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"acls": []any{}, "total_count": 0})
}

func (app *Application) UpdateFrontend(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateFrontend(chi.URLParam(r, "frontend_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
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
	// Prefer lb_id from URL path (nested route) over body.
	if lbID := chi.URLParam(r, "lb_id"); lbID != "" {
		body["lb_id"] = lbID
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

func (app *Application) ListBackends(w http.ResponseWriter, r *http.Request) {
	lbID := chi.URLParam(r, "lb_id")
	var items []map[string]any
	var err error
	if lbID != "" {
		items, err = app.repo.ListBackendsByLB(lbID)
	} else {
		items, err = app.repo.ListBackends()
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "backends", items)
}

func (app *Application) UpdateBackend(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateBackend(chi.URLParam(r, "backend_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
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
	lbID := chi.URLParam(r, "lb_id")
	pnID, _ := body["private_network_id"].(string)
	out, err := app.repo.AttachLBPrivateNetwork(lbID, pnID)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	// Include the LB object - the provider accesses pn.LB.ID after attach.
	lb, err := app.repo.GetLB(lbID)
	if err == nil {
		out["lb"] = lb
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListLBPrivateNetworks(w http.ResponseWriter, r *http.Request) {
	lbID := chi.URLParam(r, "lb_id")
	items, err := app.repo.ListLBPrivateNetworks(lbID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// Enrich each item with the LB object — the provider accesses pn.LB.Zone.
	lb, lbErr := app.repo.GetLB(lbID)
	if lbErr == nil {
		for i := range items {
			items[i]["lb"] = lb
		}
	}
	writeList(w, "private_network", items)
}

func (app *Application) DeleteLBPrivateNetwork(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteLBPrivateNetwork(chi.URLParam(r, "lb_id"), chi.URLParam(r, "pn_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}
