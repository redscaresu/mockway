package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *Application) ListProductsServers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"servers": map[string]any{
			"DEV1-S": map[string]any{
				"monthly_price":      11.99,
				"hourly_price":       0.018,
				"ncpus":              2,
				"ram":                2147483648,
				"arch":               "x86_64",
				"volumes_constraint": map[string]any{"min_size": 10000000000, "max_size": 20000000000},
			},
			"DEV1-M": map[string]any{
				"monthly_price":      23.99,
				"hourly_price":       0.036,
				"ncpus":              3,
				"ram":                4294967296,
				"arch":               "x86_64",
				"volumes_constraint": map[string]any{"min_size": 10000000000, "max_size": 40000000000},
			},
			"GP1-XS": map[string]any{
				"monthly_price":      39.99,
				"hourly_price":       0.06,
				"ncpus":              4,
				"ram":                8589934592,
				"arch":               "x86_64",
				"volumes_constraint": map[string]any{"min_size": 20000000000, "max_size": 150000000000},
			},
			"GP1-S": map[string]any{
				"monthly_price":      59.99,
				"hourly_price":       0.09,
				"ncpus":              8,
				"ram":                17179869184,
				"arch":               "x86_64",
				"volumes_constraint": map[string]any{"min_size": 20000000000, "max_size": 300000000000},
			},
			"GP1-M": map[string]any{
				"monthly_price":      119.99,
				"hourly_price":       0.18,
				"ncpus":              16,
				"ram":                34359738368,
				"arch":               "x86_64",
				"volumes_constraint": map[string]any{"min_size": 50000000000, "max_size": 600000000000},
			},
		},
	})
}

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
	writeJSON(w, http.StatusOK, map[string]any{"server": out})
}

func (app *Application) GetServer(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetServer(chi.URLParam(r, "server_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"server": out})
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
	writeJSON(w, http.StatusOK, map[string]any{"ip": out})
}

func (app *Application) GetIP(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetIP(chi.URLParam(r, "ip_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ip": out})
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
	writeJSON(w, http.StatusOK, map[string]any{"security_group": out})
}

func (app *Application) GetSecurityGroup(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetSecurityGroup(chi.URLParam(r, "sg_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"security_group": out})
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

func (app *Application) UpdateSecurityGroup(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateSecurityGroup(chi.URLParam(r, "sg_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"security_group": out})
}

func (app *Application) SetSecurityGroupRules(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	rules, ok := body["rules"]
	if !ok {
		rules = body
	}
	if _, err := app.repo.SetSecurityGroupRules(chi.URLParam(r, "sg_id"), rules); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func (app *Application) GetSecurityGroupRules(w http.ResponseWriter, r *http.Request) {
	rules, err := app.repo.GetSecurityGroupRules(chi.URLParam(r, "sg_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rules":       rules,
		"total_count": len(rules),
	})
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
	writeJSON(w, http.StatusOK, map[string]any{"private_nic": out})
}

func (app *Application) GetPrivateNIC(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetPrivateNIC(chi.URLParam(r, "nic_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"private_nic": out})
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
