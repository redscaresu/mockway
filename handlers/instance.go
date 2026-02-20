package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *Application) CreateServer(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	zone := chi.URLParam(r, "zone")
	out, err := app.repo.CreateServer(zone, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetServer(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetServer(chi.URLParam(r, "server_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListServers(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListServers(chi.URLParam(r, "zone"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "servers", items)
}

func (app *Application) DeleteServer(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteServer(chi.URLParam(r, "server_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateIP(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateIP(chi.URLParam(r, "zone"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetIP(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetIP(chi.URLParam(r, "ip_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListIPs(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListIPs(chi.URLParam(r, "zone"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "ips", items)
}

func (app *Application) DeleteIP(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteIP(chi.URLParam(r, "ip_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateSecurityGroup(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateSecurityGroup(chi.URLParam(r, "zone"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetSecurityGroup(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetSecurityGroup(chi.URLParam(r, "sg_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListSecurityGroups(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListSecurityGroups(chi.URLParam(r, "zone"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "security_groups", items)
}

func (app *Application) DeleteSecurityGroup(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteSecurityGroup(chi.URLParam(r, "sg_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreatePrivateNIC(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreatePrivateNIC(chi.URLParam(r, "zone"), chi.URLParam(r, "server_id"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetPrivateNIC(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetPrivateNIC(chi.URLParam(r, "nic_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListPrivateNICs(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListPrivateNICsByServer(chi.URLParam(r, "server_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "private_nics", items)
}

func (app *Application) DeletePrivateNIC(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeletePrivateNIC(chi.URLParam(r, "nic_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}
