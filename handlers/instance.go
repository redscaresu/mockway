package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (app *Application) ListProductsServers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"servers": map[string]any{
			"DEV1-S": map[string]any{
				"monthly_price":       11.99,
				"hourly_price":        0.018,
				"ncpus":               2,
				"ram":                 2147483648,
				"arch":                "x86_64",
				"volume_type":         "l_ssd",
				"default_volume_type": "l_ssd",
				"volumes_constraint":  map[string]any{"min_size": 0, "max_size": 20000000000},
				"per_volume_constraint": map[string]any{
					"l_ssd": map[string]any{"min_size": 0, "max_size": 20000000000},
				},
			},
			"DEV1-M": map[string]any{
				"monthly_price":       23.99,
				"hourly_price":        0.036,
				"ncpus":               3,
				"ram":                 4294967296,
				"arch":                "x86_64",
				"volume_type":         "l_ssd",
				"default_volume_type": "l_ssd",
				"volumes_constraint":  map[string]any{"min_size": 0, "max_size": 40000000000},
				"per_volume_constraint": map[string]any{
					"l_ssd": map[string]any{"min_size": 0, "max_size": 40000000000},
				},
			},
			"GP1-XS": map[string]any{
				"monthly_price":       39.99,
				"hourly_price":        0.06,
				"ncpus":               4,
				"ram":                 8589934592,
				"arch":                "x86_64",
				"volume_type":         "l_ssd",
				"default_volume_type": "l_ssd",
				"volumes_constraint":  map[string]any{"min_size": 0, "max_size": 150000000000},
				"per_volume_constraint": map[string]any{
					"l_ssd": map[string]any{"min_size": 0, "max_size": 150000000000},
				},
			},
			"GP1-S": map[string]any{
				"monthly_price":       59.99,
				"hourly_price":        0.09,
				"ncpus":               8,
				"ram":                 17179869184,
				"arch":                "x86_64",
				"volume_type":         "l_ssd",
				"default_volume_type": "l_ssd",
				"volumes_constraint":  map[string]any{"min_size": 0, "max_size": 300000000000},
				"per_volume_constraint": map[string]any{
					"l_ssd": map[string]any{"min_size": 0, "max_size": 300000000000},
				},
			},
			"GP1-M": map[string]any{
				"monthly_price":       119.99,
				"hourly_price":        0.18,
				"ncpus":               16,
				"ram":                 34359738368,
				"arch":                "x86_64",
				"volume_type":         "l_ssd",
				"default_volume_type": "l_ssd",
				"volumes_constraint":  map[string]any{"min_size": 0, "max_size": 600000000000},
				"per_volume_constraint": map[string]any{
					"l_ssd": map[string]any{"min_size": 0, "max_size": 600000000000},
				},
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
	normalizeServerSecurityGroup(body)
	normalizeServerImage(body, zone)
	out, err := app.repo.CreateServer(zone, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"server": out})
}

func normalizeServerSecurityGroup(body map[string]any) {
	if raw, ok := body["security_group"]; ok {
		switch v := raw.(type) {
		case string:
			id := strings.TrimSpace(v)
			if id == "" {
				body["security_group"] = nil
				delete(body, "security_group_id")
				return
			}
			body["security_group"] = map[string]any{"id": id, "name": ""}
			body["security_group_id"] = id
			return
		case map[string]any:
			id, _ := v["id"].(string)
			id = strings.TrimSpace(id)
			if id == "" {
				body["security_group"] = nil
				delete(body, "security_group_id")
				return
			}
			if _, ok := v["name"]; !ok {
				v["name"] = ""
			}
			body["security_group"] = v
			body["security_group_id"] = id
			return
		case nil:
			body["security_group"] = nil
			delete(body, "security_group_id")
			return
		default:
			body["security_group"] = nil
			delete(body, "security_group_id")
			return
		}
	}

	if rawID, ok := body["security_group_id"].(string); ok {
		id := strings.TrimSpace(rawID)
		if id != "" {
			body["security_group"] = map[string]any{"id": id, "name": ""}
			body["security_group_id"] = id
			return
		}
	}

	body["security_group"] = nil
	delete(body, "security_group_id")
}

func normalizeServerImage(body map[string]any, zone string) {
	raw, ok := body["image"]
	if !ok {
		return
	}

	imageRef, ok := raw.(string)
	if !ok {
		return
	}

	imageRef = strings.TrimSpace(imageRef)
	if imageRef == "" {
		return
	}

	imageID := imageRef
	if _, err := uuid.Parse(imageRef); err != nil {
		imageID = localImageID(imageRef, zone, "instance_sbs")
	}

	body["image"] = map[string]any{
		"id":                 imageID,
		"name":               imageRef,
		"arch":               "x86_64",
		"default_bootscript": map[string]any{},
		"from_server":        "",
		"organization":       "",
		"public":             false,
		"root_volume":        map[string]any{},
		"extra_volumes":      map[string]any{},
	}
}

func (app *Application) GetServer(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetServer(chi.URLParam(r, "server_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"server": out})
}

func (app *Application) ListServerUserData(w http.ResponseWriter, r *http.Request) {
	if _, err := app.repo.GetServer(chi.URLParam(r, "server_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_data": []string{}})
}

func (app *Application) ServerAction(w http.ResponseWriter, r *http.Request) {
	if _, err := app.repo.GetServer(chi.URLParam(r, "server_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	action, _ := body["action"].(string)
	writeJSON(w, http.StatusOK, map[string]any{
		"task": map[string]any{
			"id":          uuid.NewString(),
			"description": action,
			"progress":    100,
			"status":      "success",
		},
	})
}

func (app *Application) SetServerUserData(w http.ResponseWriter, r *http.Request) {
	if _, err := app.repo.GetServer(chi.URLParam(r, "server_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	defer r.Body.Close()
	writeNoContent(w)
}

func (app *Application) GetVolume(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetInstanceVolume(chi.URLParam(r, "zone"), chi.URLParam(r, "volume_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"volume": out})
}

func (app *Application) DeleteVolume(w http.ResponseWriter, _ *http.Request) {
	writeNoContent(w)
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
