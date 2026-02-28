package handlers

import (
	"net/http"
)

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

	writeJSON(w, http.StatusOK, map[string]any{"ips": []any{}, "total_count": 0})
}
