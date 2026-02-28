package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (app *Application) ListDNSZones(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		domain = "example.com"
	}
	zones := []map[string]any{{
		"domain":     domain,
		"subdomain":  "",
		"ns":         []string{"ns0.dom.scw.cloud", "ns1.dom.scw.cloud"},
		"ns_default": []string{"ns0.dom.scw.cloud", "ns1.dom.scw.cloud"},
		"ns_master":  []string{},
		"status":     "active",
		"project_id": "00000000-0000-0000-0000-000000000000",
	}}
	// If a subdomain is requested, also include a matching zone.
	if dns := r.URL.Query().Get("dns_zone"); dns != "" {
		parts := strings.SplitN(dns, ".", 2)
		if len(parts) == 2 {
			zones = append(zones, map[string]any{
				"domain":     parts[1],
				"subdomain":  parts[0],
				"ns":         []string{"ns0.dom.scw.cloud", "ns1.dom.scw.cloud"},
				"ns_default": []string{"ns0.dom.scw.cloud", "ns1.dom.scw.cloud"},
				"ns_master":  []string{},
				"status":     "active",
				"project_id": "00000000-0000-0000-0000-000000000000",
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"dns_zones":   zones,
		"total_count": len(zones),
	})
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
