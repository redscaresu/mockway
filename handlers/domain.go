package handlers

import (
	"net/http"
	"strings"

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

func (app *Application) ListDNSZones(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	dnsZone := r.URL.Query().Get("dns_zone")

	zones, err := app.repo.ListDNSZones(domain)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// Filter by dns_zone if requested. Narrow to matching zones and synthesize
	// a subdomain zone when the parent domain exists in storage.
	if dnsZone != "" {
		parts := strings.SplitN(dnsZone, ".", 2)
		filtered := make([]map[string]any, 0)
		hasExactMatch := false
		hasParentDomain := false
		for _, z := range zones {
			sub, _ := z["subdomain"].(string)
			d, _ := z["domain"].(string)
			full := d
			if sub != "" {
				full = sub + "." + d
			}
			if full == dnsZone {
				hasExactMatch = true
				filtered = append(filtered, z)
			}
		}
		if !hasExactMatch {
			for _, z := range zones {
				sub, _ := z["subdomain"].(string)
				d, _ := z["domain"].(string)
				if len(parts) == 2 && d == parts[1] && sub == "" {
					hasParentDomain = true
					filtered = append(filtered, z)
				}
			}
		}
		if !hasExactMatch && hasParentDomain && len(parts) == 2 && parts[0] != "" {
			filtered = append(filtered, map[string]any{
				"domain":     parts[1],
				"subdomain":  parts[0],
				"ns":         []any{"ns0.dom.scw.cloud", "ns1.dom.scw.cloud"},
				"ns_default": []any{"ns0.dom.scw.cloud", "ns1.dom.scw.cloud"},
				"ns_master":  []any{},
				"status":     "active",
				"project_id": "00000000-0000-0000-0000-000000000000",
			})
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
