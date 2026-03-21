package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func lbScope(r *http.Request) string {
	if z := chi.URLParam(r, "zone"); z != "" {
		return z
	}
	return chi.URLParam(r, "region")
}

func (app *Application) GetLBACL(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetLBACL(chi.URLParam(r, "acl_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) CreateLBACL(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	frontendID := chi.URLParam(r, "frontend_id")
	out, err := app.repo.CreateLBACL(frontendID, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) UpdateLBACL(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateLBACL(chi.URLParam(r, "acl_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteLBACL(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteLBACL(chi.URLParam(r, "acl_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateLBRoute(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	// Validate frontend_id and backend_id references exist.
	var lbID string
	if frontendID, _ := body["frontend_id"].(string); frontendID != "" {
		fe, err := app.repo.GetFrontend(frontendID)
		if err != nil {
			writeCreateError(w, err)
			return
		}
		lbID, _ = fe["lb_id"].(string)
	}
	if backendID, _ := body["backend_id"].(string); backendID != "" {
		if _, err := app.repo.GetBackend(backendID); err != nil {
			writeCreateError(w, err)
			return
		}
	}
	out, err := app.repo.CreateLBRoute(lbID, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetLBRoute(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetLBRoute(chi.URLParam(r, "route_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListLBRoutes(w http.ResponseWriter, r *http.Request) {
	lbID := r.URL.Query().Get("lb_id")
	frontendID := r.URL.Query().Get("frontend_id")
	items, err := app.repo.ListLBRoutes(lbID, frontendID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "routes", items)
}

func (app *Application) UpdateLBRoute(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateLBRoute(chi.URLParam(r, "route_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteLBRoute(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteLBRoute(chi.URLParam(r, "route_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateLBCertificate(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	lbID := chi.URLParam(r, "lb_id")
	out, err := app.repo.CreateLBCertificate(lbID, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetLBCertificate(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetLBCertificate(chi.URLParam(r, "certificate_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListLBCertificates(w http.ResponseWriter, r *http.Request) {
	lbID := chi.URLParam(r, "lb_id")
	items, err := app.repo.ListLBCertificatesByLB(lbID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "certificates", items)
}

func (app *Application) UpdateLBCertificate(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateLBCertificate(chi.URLParam(r, "certificate_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteLBCertificate(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteLBCertificate(chi.URLParam(r, "certificate_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) UpdateLBHealthCheck(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateBackend(chi.URLParam(r, "backend_id"), map[string]any{"health_check": body})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) SetLBBackendServers(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	servers, _ := body["server_ip"].([]any)
	out, err := app.repo.UpdateBackend(chi.URLParam(r, "backend_id"), map[string]any{"server_ip": servers})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) MigrateLB(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetLB(chi.URLParam(r, "lb_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
