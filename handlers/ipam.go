package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *Application) CreateIPAMIP(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateIPAMIP(chi.URLParam(r, "region"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetIPAMIP(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetIPAMIP(chi.URLParam(r, "ip_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) UpdateIPAMIP(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateIPAMIP(chi.URLParam(r, "ip_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteIPAMIP(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteIPAMIP(chi.URLParam(r, "ip_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) DetachIPAMIP(w http.ResponseWriter, r *http.Request) {
	if _, err := app.repo.GetIPAMIP(chi.URLParam(r, "ip_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (app *Application) MoveIPAMIP(w http.ResponseWriter, r *http.Request) {
	if _, err := app.repo.GetIPAMIP(chi.URLParam(r, "ip_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

// ListIPAMIPs returns synthetic IPAM IPs for private NICs.
// The Scaleway TF provider queries this to populate private_ips on NICs.
func (app *Application) ListIPAMIPs(w http.ResponseWriter, r *http.Request) {
	resourceID := r.URL.Query().Get("resource_id")
	resourceType := r.URL.Query().Get("resource_type")

	if resourceType == "instance_private_nic" && resourceID != "" {
		nic, err := app.repo.GetPrivateNIC(resourceID)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ips": []any{}, "total_count": 0})
			return
		}
		privIPs, _ := nic["private_ips"].([]any)
		ips := make([]any, 0, len(privIPs))
		for _, raw := range privIPs {
			pip, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			addr, _ := pip["address"].(string)
			ips = append(ips, map[string]any{
				"id":      pip["id"],
				"address": addr,
				"resource": map[string]any{
					"id":   resourceID,
					"type": resourceType,
				},
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"ips": ips, "total_count": len(ips)})
		return
	}

	// Fall through to stored IPAM IPs for normal (non-NIC) queries.
	region := chi.URLParam(r, "region")
	items, err := app.repo.ListIPAMIPs(region)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "ips", items)
}
