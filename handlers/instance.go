package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redscaresu/mockway/models"
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
			"GP1-L": map[string]any{
				"monthly_price":       239.99,
				"hourly_price":        0.36,
				"ncpus":               32,
				"ram":                 68719476736,
				"arch":                "x86_64",
				"volume_type":         "l_ssd",
				"default_volume_type": "l_ssd",
				"volumes_constraint":  map[string]any{"min_size": 0, "max_size": 600000000000},
				"per_volume_constraint": map[string]any{
					"l_ssd": map[string]any{"min_size": 0, "max_size": 600000000000},
				},
			},
			"GP1-XL": map[string]any{
				"monthly_price":       479.99,
				"hourly_price":        0.72,
				"ncpus":               48,
				"ram":                 137438953472,
				"arch":                "x86_64",
				"volume_type":         "l_ssd",
				"default_volume_type": "l_ssd",
				"volumes_constraint":  map[string]any{"min_size": 0, "max_size": 600000000000},
				"per_volume_constraint": map[string]any{
					"l_ssd": map[string]any{"min_size": 0, "max_size": 600000000000},
				},
			},
			"DEV1-L": map[string]any{
				"monthly_price":       47.99,
				"hourly_price":        0.072,
				"ncpus":               4,
				"ram":                 8589934592,
				"arch":                "x86_64",
				"volume_type":         "l_ssd",
				"default_volume_type": "l_ssd",
				"volumes_constraint":  map[string]any{"min_size": 0, "max_size": 80000000000},
				"per_volume_constraint": map[string]any{
					"l_ssd": map[string]any{"min_size": 0, "max_size": 80000000000},
				},
			},
		},
	})
}

// CRITICAL[instance-root-volume-boot-default-false]: the root volume's
// `boot` flag is FALSE by default (scaleway.instance.v1: "Force the
// Instance to boot on this volume", default: false).
//
// Returning true made the provider read back a value its own schema
// defaults to false, so `tofu plan` proposed `boot = true -> false` on
// every run and no stack with an instance ever converged. The apply
// succeeded every time, which is why nothing caught it until a second
// plan was run. Implemented in repository.CreateServer.
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
		// Validate the label is a known marketplace image — reject typos.
		known := false
		for _, l := range knownMarketplaceLabels {
			if l == imageRef {
				known = true
				break
			}
		}
		if !known {
			// Unknown label — leave as-is so the provider's marketplace
			// lookup returns empty and fails with a clear error.
			return
		}
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

func (app *Application) UpdateServer(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateServer(chi.URLParam(r, "server_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"server": out})
}

// CRITICAL[instance-user-data-round-trips]: the keys written by PATCH
// must be the keys listed here. Returning an empty list made the
// provider see no cloud-init on read, propose adding it again, and
// never converge.
func (app *Application) ListServerUserData(w http.ResponseWriter, r *http.Request) {
	keys, err := app.repo.ListServerUserDataKeys(chi.URLParam(r, "server_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_data": keys})
}

// GetServerUserDataKey handles GET /servers/{server_id}/user_data/{key}
// and returns the value as text/plain, which is how the real API serves
// it -- the body IS the value, not a JSON envelope.
//
// CRITICAL[instance-user-data-round-trips].
func (app *Application) GetServerUserDataKey(w http.ResponseWriter, r *http.Request) {
	value, err := app.repo.GetServerUserData(chi.URLParam(r, "server_id"), chi.URLParam(r, "key"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(value))
}

func (app *Application) ServerAction(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "server_id")
	if _, err := app.repo.GetServer(serverID); err != nil {
		writeDomainError(w, err)
		return
	}
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	action, _ := body["action"].(string)
	switch action {
	case "terminate":
		if err := app.repo.DeleteServer(serverID); err != nil {
			writeDomainError(w, err)
			return
		}
	case "poweron":
		if err := app.repo.SetServerState(serverID, "running"); err != nil {
			writeDomainError(w, err)
			return
		}
	case "poweroff", "stop_in_place":
		state := "stopped"
		if action == "stop_in_place" {
			state = "stopped_in_place"
		}
		if err := app.repo.SetServerState(serverID, state); err != nil {
			writeDomainError(w, err)
			return
		}
	case "reboot":
		if err := app.repo.SetServerState(serverID, "running"); err != nil {
			writeDomainError(w, err)
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "unknown action: " + action, "type": "invalid_argument"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"task": map[string]any{
			"id":          uuid.NewString(),
			"description": action,
			"progress":    100,
			"status":      "success",
		},
	})
}

// SetServerUserData stores the value so the matching GET can return it.
//
// The body is the raw value, not JSON: the provider PATCHes cloud-init
// as text/plain. CRITICAL[instance-user-data-round-trips].
func (app *Application) SetServerUserData(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	value, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "unreadable body", "type": "invalid_argument"})
		return
	}
	if err := app.repo.SetServerUserData(
		chi.URLParam(r, "server_id"), chi.URLParam(r, "key"), string(value)); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

// DeleteServerUserDataKey handles DELETE on a user_data key, which the
// provider calls when cloud-init is removed from the configuration. A
// key that is not there is already in the desired state, so this is
// idempotent rather than a 404.
func (app *Application) DeleteServerUserDataKey(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "server_id")
	if _, err := app.repo.GetServer(serverID); err != nil {
		writeDomainError(w, err)
		return
	}
	if err := app.repo.DeleteServerUserData(serverID, chi.URLParam(r, "key")); err != nil {
		writeDomainError(w, err)
		return
	}
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

func (app *Application) DeleteVolume(w http.ResponseWriter, r *http.Request) {
	// Standalone volumes can be deleted; embedded server volumes are no-op.
	id := chi.URLParam(r, "volume_id")
	if err := app.repo.DeleteStandaloneVolume(id); err != nil && !errors.Is(err, models.ErrNotFound) {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateVolume(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateInstanceVolume(chi.URLParam(r, "zone"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"volume": out})
}

func (app *Application) ListVolumes(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListInstanceVolumes(chi.URLParam(r, "zone"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "volumes", items)
}

func (app *Application) PatchVolume(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateInstanceVolume(chi.URLParam(r, "volume_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"volume": out})
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

func (app *Application) UpdateInstanceIP(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateIP(chi.URLParam(r, "ip_id"), body)
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
	splitSecurityGroupRules(out)
	writeJSON(w, http.StatusOK, map[string]any{"security_group": out})
}

// splitSecurityGroupRules transforms the stored flat "rules" array (each entry
// has a "direction" field) into separate "inbound_rule" and "outbound_rule"
// arrays, which is the shape the Scaleway Terraform provider expects when it
// refreshes a scaleway_instance_security_group resource via GET.
func splitSecurityGroupRules(sg map[string]any) {
	rules, _ := sg["rules"].([]any)
	inbound := make([]any, 0)
	outbound := make([]any, 0)
	for _, r := range rules {
		rule, ok := r.(map[string]any)
		if !ok {
			continue
		}
		switch rule["direction"] {
		case "inbound":
			inbound = append(inbound, rule)
		case "outbound":
			outbound = append(outbound, rule)
		}
	}
	sg["inbound_rule"] = inbound
	sg["outbound_rule"] = outbound
	delete(sg, "rules")
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
	// Validate the NIC belongs to the server in the URL path.
	if serverID := chi.URLParam(r, "server_id"); serverID != "" {
		if nicServer, _ := out["server_id"].(string); nicServer != serverID {
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "resource not found", "type": "not_found"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"private_nic": out})
}

func (app *Application) ListPrivateNICs(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "server_id")
	if _, err := app.repo.GetServer(serverID); err != nil {
		writeDomainError(w, err)
		return
	}
	items, err := app.repo.ListPrivateNICsByServer(serverID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "private_nics", items)
}

func (app *Application) DeletePrivateNIC(w http.ResponseWriter, r *http.Request) {
	nicID := chi.URLParam(r, "nic_id")
	// Validate the NIC belongs to the server in the URL path.
	if serverID := chi.URLParam(r, "server_id"); serverID != "" {
		nic, err := app.repo.GetPrivateNIC(nicID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		if nicServer, _ := nic["server_id"].(string); nicServer != serverID {
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "resource not found", "type": "not_found"})
			return
		}
	}
	if err := app.repo.DeletePrivateNIC(nicID); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

// ListPrivateNetworkInterfacesV2 answers the v2alpha1 endpoint the
// Scaleway provider polls while creating a server.
//
// CRITICAL[instance-v2-pnis-envelope]: the provider calls this on every
// scaleway_instance_server create, so a 501 here fails the apply before
// any of the server's own behaviour is exercised -- which is exactly how
// it surfaced: infrafactory's lb-serving-paris could not clear Layer 2,
// and Layer 2 gates Layer 3.
//
// The envelope was read off the real API rather than guessed:
//
//	GET /instance/v2alpha1/zones/fr-par-1/private-network-interfaces
//	200 {"private_network_interfaces": [], "total_count": 0}
//
// CRITICAL[instance-v2-pnis-filters-by-server]: the provider passes
// server_id, so an unfiltered listing would report another server's
// interfaces as this one's. The listing is also confined to the zone in
// the request path, because the server_id lookup is not itself zonal.
//
// CRITICAL[instance-v2-pnis-scoped-to-zone]: the server_id lookup is not
// zonal, so an unfiltered result returns another zone's interfaces.
//
// An unknown server_id returns 200 with an empty list rather than 404.
// That looks like a missing existence check and is not one -- the real
// API was asked:
//
//	GET .../private-network-interfaces?server_id=00000000-0000-0000-0000-000000000000
//	200 {"private_network_interfaces": [], "total_count": 0}
//
// The element shape is inferred: it carries the v1 private-NIC records,
// which hold the same identifying fields. No real populated response has
// been observed, so a scenario that both attaches a private network and
// asserts on this endpoint's element fields is not yet trustworthy here.
func (app *Application) ListPrivateNetworkInterfacesV2(w http.ResponseWriter, r *http.Request) {
	zone := chi.URLParam(r, "zone")
	serverID := r.URL.Query().Get("server_id")

	var (
		items []map[string]any
		err   error
	)
	if serverID != "" {
		items, err = app.repo.ListPrivateNICsByServer(serverID)
	} else {
		items, err = app.repo.ListPrivateNICsByZone(zone)
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "private_network_interfaces", filterByZone(items, zone))
}

// filterByZone keeps the listing inside the zone in the request path.
//
// The server_id lookup is not zonal, so without this a NIC on a server
// in fr-par-2 would come back from a fr-par-1 request.
func filterByZone(items []map[string]any, zone string) []map[string]any {
	kept := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item["zone"] == zone {
			kept = append(kept, item)
		}
	}
	return kept
}

// CreatePrivateNetworkInterfaceV2 attaches a server to a private network.
//
// CRITICAL[instance-v2-pni-create]: the Scaleway provider creates
// `scaleway_instance_private_nic` through this endpoint, not through the
// v1 `/servers/{id}/private_nics` route. Returning 501 here fails the
// apply, which stops Layer 2, which stops Layer 3 -- infrafactory runs
// Layer 3 only when the mock apply succeeded. So a missing verb on this
// one path blocked every Scaleway compute scenario from ever reaching
// real cloud through `test`.
//
// Unlike v1, the server is named in the BODY rather than the path: this
// route is zone-scoped only. Taking it from the body is therefore not a
// shortcut, it is the shape of the endpoint.
//
// CRITICAL[instance-v2-pni-unwrapped]: the response is the object
// ITSELF, with no envelope -- v1 wraps in `private_nic` and the v2
// listing wraps in `private_network_interfaces`, so a singular
// `private_network_interface` envelope is the natural guess and it is
// wrong. Read off the SDK rather than guessed:
//
//	var resp PrivateNetworkInterface
//	err = s.client.Do(scwReq, &resp, opts...)
//
// An envelope here parses as an empty object, so the provider's next call
// fails with "field PrivateNetworkInterfaceID cannot be empty in
// request" -- a message that points at the request and not at this
// response, which is what makes the mistake expensive to find.
//
// Backed by the same repository records as the v1 route, so a NIC created
// here is visible to `GET /servers/{id}/private_nics` and vice versa. Two
// stores would let a server report different interfaces depending on
// which API version asked. The record therefore carries BOTH spellings of
// the same field -- v1's `state` and v2's `status`.
//
// One observed divergence from real Scaleway, recorded rather than
// reconciled: on 2026-08-31 a real NIC came back with `private_ips: null`
// immediately after create, while mockway synthesises one. That is a
// single sample taken seconds after create, before IPAM would have had
// time to attach anything, so it is not enough to call the real steady
// state -- and guessing from it would trade a known small divergence for
// an unknown one.
func (app *Application) CreatePrivateNetworkInterfaceV2(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}

	serverID, _ := body["server_id"].(string)
	if serverID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"message": "server_id is required", "type": "invalid_argument",
		})
		return
	}
	// Checked rather than assumed: creating an interface on a server that
	// does not exist would leave a record nothing can reach, and the v1
	// listing route makes the same check.
	//
	// Reported as a CREATE error, not a domain one. On this path the
	// server is a resource being REFERENCED, so the body says
	// `referenced server "<id>" not found` -- and the private-network
	// foreign key inside CreatePrivateNIC below already answers that way.
	// Two spellings for the same missing server, depending on which check
	// happened to catch it first, is the kind of inconsistency a client
	// cannot code against.
	server, err := app.repo.GetServer(serverID)
	if err != nil {
		writeCreateErrorFor(w, err, "server", serverID)
		return
	}

	// CRITICAL[instance-v2-pni-zone-scoped]: this route is zone-scoped
	// but the server_id lookup is not, so nothing else stops an interface
	// being created in one zone against a server in another. The stored
	// record would then claim a zone its server is not in, and
	// ListPrivateNetworkInterfacesV2 -- which filters by zone precisely
	// because of this -- would hide it from both.
	zone := chi.URLParam(r, "zone")
	if serverZone, _ := server["zone"].(string); serverZone != "" && serverZone != zone {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"message":     fmt.Sprintf("referenced server %q not found in zone %s", serverID, zone),
			"type":        "not_found",
			"resource":    "server",
			"resource_id": serverID,
		})
		return
	}

	out, err := app.repo.CreatePrivateNIC(zone, serverID, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// GetPrivateNetworkInterfaceV2 reads one interface by its own id.
//
// No server_id in the path to cross-check against, unlike the v1 route --
// the id is the whole address here.
func (app *Application) GetPrivateNetworkInterfaceV2(w http.ResponseWriter, r *http.Request) {
	out, err := app.privateNetworkInterfaceInZone(chi.URLParam(r, "zone"), chi.URLParam(r, "pni_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// UpdatePrivateNetworkInterfaceV2 changes an interface's tags in place.
//
// The provider updates tags in place rather than replacing the interface
// -- confirmed by planning a tag change against the mock, which reported
// "will be updated in-place" and then failed the apply with a 501. So a
// missing PATCH is not a gap in coverage, it is a broken apply.
func (app *Application) UpdatePrivateNetworkInterfaceV2(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	if _, err := app.privateNetworkInterfaceInZone(chi.URLParam(r, "zone"), chi.URLParam(r, "pni_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	out, err := app.repo.UpdatePrivateNIC(chi.URLParam(r, "pni_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// privateNetworkInterfaceInZone reads an interface and refuses one that
// lives in another zone.
//
// CRITICAL[instance-v2-pni-zone-scoped]: every v2alpha1 interface route
// is zone-scoped, so an id lookup that ignores the zone lets a client
// with the wrong zone read, update or DELETE an interface belonging to
// another one -- and against a mock that wrong client passes, which is
// the whole failure mode a fidelity mock exists to prevent. The listing
// route already filters for exactly this reason.
func (app *Application) privateNetworkInterfaceInZone(zone, id string) (map[string]any, error) {
	out, err := app.repo.GetPrivateNIC(id)
	if err != nil {
		return nil, err
	}
	if nicZone, _ := out["zone"].(string); nicZone != zone {
		return nil, models.ErrNotFound
	}
	return out, nil
}

// DeletePrivateNetworkInterfaceV2 detaches the server from the network.
//
// The provider destroys through this route, so without it a scenario
// could be applied and never torn down -- which against a mock means the
// next run inherits it, and is exactly the class of leak the orphan
// sweep exists to catch.
func (app *Application) DeletePrivateNetworkInterfaceV2(w http.ResponseWriter, r *http.Request) {
	// Zone-checked BEFORE deleting: a wrong-zone request must not be able
	// to remove another zone's interface, which is the one mistake here
	// that cannot be undone by retrying.
	if _, err := app.privateNetworkInterfaceInZone(chi.URLParam(r, "zone"), chi.URLParam(r, "pni_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	// CRITICAL[nic-delete-v2alpha1-always-refuses]: this route ALWAYS
	// refuses, with 412 and this exact prose. Real Scaleway, 2026-09-10:
	//
	//	DELETE /instance/v2alpha1/zones/fr-par-1/private-network-interfaces/{id}
	//	412 {"precondition":"resource_not_usable",
	//	     "help_message":"Can't delete a private network interface attached to a server"}
	//
	// A private NIC is by definition attached to a server, so the
	// precondition can never be satisfied here. Power state is NOT the
	// variable -- the same NIC on the same server deleted through v1
	// seconds later with 204, and again later on a RUNNING server.
	//
	// This matters more than a usual fidelity gap because provider
	// 2.81.0 destroys NICs through THIS route, so it can never tear one
	// down. infrafactory works around it by deleting through v1 first
	// (ADR-0031); a mock that let v2alpha1 succeed would make that
	// workaround look unnecessary, which is how the defect survived two
	// days of investigation.
	writeJSON(w, http.StatusPreconditionFailed, map[string]any{
		"message":      "precondition is not respected",
		"help_message": "Can't delete a private network interface attached to a server",
		"precondition": "resource_not_usable",
		"type":         "precondition_failed",
	})
}

// DetachPrivateNetworkInterfaceV2 handles
// POST /instance/v2alpha1/zones/{zone}/servers/{server_id}/detach-private-network-interface
//
// CRITICAL[instance-v2-detach-pni]: provider 2.83.0 tears a private
// network attachment down through THIS route, naming the interface in
// `private_network_interface_id`. 2.81.0 used
// `DELETE .../private-network-interfaces/{id}`, which always refuses
// (CRITICAL[nic-delete-v2alpha1-always-refuses]) and is why teardown
// was broken for two days -- upstream fixed it in
// scaleway/terraform-provider-scaleway#4354 by changing the endpoint.
// A mock without this route answers 501 and every compute teardown
// fails against it.
//
// The field name was READ OFF THE WIRE, not guessed: there is no
// published v2alpha1 spec, so provider 2.83.0 driving this route is the
// only source for the shape. The first version of this handler assumed
// `private_network_id` and still passed, because it fell back to
// detaching every interface on the server when the id was absent -- a
// mock permissive enough to accept a request naming nothing, which is
// the failure mode a fidelity mock exists to prevent. Hence: the id is
// required, it must name an interface on THIS server in THIS zone, and
// anything else is a 404.
func (app *Application) DetachPrivateNetworkInterfaceV2(w http.ResponseWriter, r *http.Request) {
	zone := chi.URLParam(r, "zone")
	serverID := chi.URLParam(r, "server_id")
	if _, err := app.repo.GetServer(serverID); err != nil {
		writeDomainError(w, err)
		return
	}
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	nicID, _ := body["private_network_interface_id"].(string)
	if strings.TrimSpace(nicID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"message": "private_network_interface_id is required",
			"type":    "invalid_argument",
		})
		return
	}

	nics, err := app.repo.ListPrivateNICsByServer(serverID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	for _, nic := range nics {
		if id, _ := nic["id"].(string); id != nicID {
			continue
		}
		// Zone-checked like every other v2alpha1 interface route
		// (CRITICAL[instance-v2-pni-zone-scoped]).
		if nicZone, _ := nic["zone"].(string); nicZone != zone {
			break
		}
		if err := app.repo.DeletePrivateNIC(nicID); err != nil {
			writeDomainError(w, err)
			return
		}
		writeDetachedServer(w, app, serverID, zone)
		return
	}

	// Not attached to this server. NOT treated as already-detached:
	// silently accepting an id this server does not own is how a mock
	// tells a client its mistake worked.
	writeDomainError(w, models.ErrNotFound)
}

// writeDetachedServer answers a detach with the updated Server.
//
// The SDK decodes this response into a Server and returns *Server
// (instance/v2alpha1: DetachServerPrivateNetworkInterface), so 204 is
// the wrong shape even though provider 2.83.0 tolerates it -- it
// leaves the caller a zero-valued Server, and any version that reads
// the result breaks against the mock while working against Scaleway.
// That is the mock being BEHIND its consumer, which is the failure
// this endpoint exists because of.
//
// v2alpha1's Server is NOT v1's. Returning the stored v1 object was the
// first attempt and the provider rejected it outright:
//
//	could not parse application/json response body: json: cannot
//	unmarshal object into Go struct field .volumes of type
//	[]*instance.ServerVolume
//
// v1 keys volumes by position in an OBJECT ({"0": {...}}); v2alpha1
// takes an ARRAY. `commercial_type` becomes `server_type`, and the
// attached networks come back as `private_network_interfaces`. So this
// projects rather than echoes -- and the response is UNWRAPPED, unlike
// v1, because the SDK decodes straight into Server instead of a
// response struct that has a Server field
// (cf. CRITICAL[instance-v2-pni-unwrapped]).
func writeDetachedServer(w http.ResponseWriter, app *Application, serverID, zone string) {
	server, err := app.repo.GetServer(serverID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	volumes := []any{}
	if stored, ok := server["volumes"].(map[string]any); ok {
		keys := make([]string, 0, len(stored))
		for k := range stored {
			keys = append(keys, k)
		}
		// Sorted, so the array order is stable across calls rather than
		// map-iteration order.
		sort.Strings(keys)
		for _, k := range keys {
			volumes = append(volumes, stored[k])
		}
	}

	nics := []any{}
	if attached, err := app.repo.ListPrivateNICsByServer(serverID); err == nil {
		for _, nic := range attached {
			nics = append(nics, nic)
		}
	}

	serverType := server["server_type"]
	if serverType == nil {
		serverType = server["commercial_type"]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":                         server["id"],
		"name":                       server["name"],
		"project_id":                 server["project"],
		"tags":                       server["tags"],
		"server_type":                serverType,
		"status":                     server["state"],
		"volumes":                    volumes,
		"private_network_interfaces": nics,
		"created_at":                 server["creation_date"],
		"updated_at":                 server["modification_date"],
		"zone":                       zone,
	})
}
