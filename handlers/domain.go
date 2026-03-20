package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *Application) CreateDNSZone(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateDNSZone(body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetDNSZone(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetDNSZone(chi.URLParam(r, "dns_zone"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListDNSZones(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	dnsZone := r.URL.Query().Get("dns_zone")

	zones, err := app.repo.ListDNSZones(domain)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// Filter by dns_zone if requested. Only return actually stored zones —
	// never synthesize zones that don't exist (would hide config bugs).
	if dnsZone != "" {
		filtered := make([]map[string]any, 0)
		for _, z := range zones {
			sub, _ := z["subdomain"].(string)
			d, _ := z["domain"].(string)
			full := d
			if sub != "" {
				full = sub + "." + d
			}
			if full == dnsZone {
				filtered = append(filtered, z)
			}
		}
		zones = filtered
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"dns_zones":   zones,
		"total_count": len(zones),
	})
}

func (app *Application) UpdateDNSZone(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateDNSZone(chi.URLParam(r, "dns_zone"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteDNSZone(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteDNSZone(chi.URLParam(r, "dns_zone")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) PatchDomainRecords(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	dnsZone := chi.URLParam(r, "dns_zone")
	changes, _ := body["changes"].([]any)
	records, err := app.repo.PatchDomainRecords(dnsZone, changes)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

func (app *Application) ListDomainRecords(w http.ResponseWriter, r *http.Request) {
	dnsZone := chi.URLParam(r, "dns_zone")
	records, err := app.repo.ListDomainRecords(dnsZone)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"records":     records,
		"total_count": len(records),
	})
}
