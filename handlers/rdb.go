package handlers

import (
	"encoding/base64"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/redscaresu/mockway/repository"
)

func (app *Application) CreateRDBInstance(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}

	if initEndpoints, ok := body["init_endpoints"]; ok {
		endpoints, err := repository.BuildRDBEndpointsFromInit(initEndpoints, body["engine"])
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid init_endpoints", "type": "invalid_argument"})
			return
		}
		if len(endpoints) > 0 {
			first, _ := endpoints[0].(map[string]any)
			if pnObj, ok := first["private_network"].(map[string]any); ok {
				pnID, _ := pnObj["id"].(string)
				exists, err := app.repo.Exists("private_networks", "id", pnID)
				if err != nil {
					writeDomainError(w, err)
					return
				}
				if !exists {
					writeJSON(w, http.StatusNotFound, map[string]any{"message": "referenced resource not found", "type": "not_found"})
					return
				}
			}
		}
		body["endpoints"] = endpoints
		delete(body, "init_endpoints")
	}

	out, err := app.repo.CreateRDBInstance(chi.URLParam(r, "region"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetRDBInstance(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetRDBInstance(chi.URLParam(r, "instance_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListRDBInstances(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListRDBInstances(chi.URLParam(r, "region"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "instances", items)
}

func (app *Application) UpdateRDBInstance(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateRDBInstance(chi.URLParam(r, "instance_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) UpgradeRDBInstance(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instance_id")
	current, err := app.repo.GetRDBInstance(instanceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	patch := map[string]any{}
	if version, ok := current["version"]; ok {
		patch["version"] = version
	}
	if engine, ok := current["engine"].(string); ok && engine != "" {
		patch["engine"] = engine
	}

	out, err := app.repo.UpdateRDBInstance(instanceID, patch)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetRDBCertificate(w http.ResponseWriter, r *http.Request) {
	if _, err := app.repo.GetRDBInstance(chi.URLParam(r, "instance_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	// The SDK's File struct has Content []byte, so JSON expects base64-encoded data.
	const pem = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALHMPMCJ+OebMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBm1v\nY2t3YTAeFw0yNDAyMjQwMDAwMDBaFw0zNDAyMjQwMDAwMDBaMBExDzANBgNVBAMM\nBm1vY2t3YTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o35FHQOGT7Pmb+oCaFHh\nOBAAPHlNmjNKHEl2hdNRMNwIDAQABMA0GCSqGSIb3DQEBCwUAA0EA\n-----END CERTIFICATE-----\n"
	writeJSON(w, http.StatusOK, map[string]any{
		"content": base64.StdEncoding.EncodeToString([]byte(pem)),
	})
}

func (app *Application) DeleteRDBInstance(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instance_id")
	out, err := app.repo.GetRDBInstance(instanceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if err := app.repo.DeleteRDBInstance(instanceID); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) CreateRDBDatabase(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	name, _ := body["name"].(string)
	out, err := app.repo.CreateRDBDatabase(chi.URLParam(r, "instance_id"), name, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListRDBDatabases(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListRDBDatabases(chi.URLParam(r, "instance_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "databases", items)
}

func (app *Application) DeleteRDBDatabase(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteRDBDatabase(chi.URLParam(r, "instance_id"), chi.URLParam(r, "db_name")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateRDBUser(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	name, _ := body["name"].(string)
	out, err := app.repo.CreateRDBUser(chi.URLParam(r, "instance_id"), name, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListRDBUsers(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListRDBUsers(chi.URLParam(r, "instance_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "users", items)
}

func (app *Application) UpdateRDBUser(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateRDBUser(chi.URLParam(r, "instance_id"), chi.URLParam(r, "user_name"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteRDBUser(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteRDBUser(chi.URLParam(r, "instance_id"), chi.URLParam(r, "user_name")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) SetRDBACLs(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	instanceID := chi.URLParam(r, "instance_id")
	rules, _ := body["rules"].([]any)
	if rules == nil {
		rules = []any{}
	}
	stored, err := app.repo.SetRDBACLs(instanceID, rules)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": stored})
}

func (app *Application) ListRDBACLs(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instance_id")
	rules, err := app.repo.ListRDBACLs(instanceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules, "total_count": len(rules)})
}

func (app *Application) DeleteRDBACLs(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instance_id")
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	var ruleIPs []string
	if acls, ok := body["acl_rules_ips"].([]any); ok {
		for _, v := range acls {
			if ip, ok := v.(string); ok {
				ruleIPs = append(ruleIPs, ip)
			}
		}
	}
	if err := app.repo.DeleteRDBACLs(instanceID, ruleIPs); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) SetRDBPrivileges(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	instanceID := chi.URLParam(r, "instance_id")
	privileges, _ := body["privileges"].([]any)
	if privileges == nil {
		privileges = []any{}
	}
	result, err := app.repo.SetRDBPrivileges(instanceID, privileges)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"privileges": result, "total_count": len(result)})
}

func (app *Application) ListRDBPrivileges(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instance_id")
	result, err := app.repo.ListRDBPrivileges(instanceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// Support filtering by database_name and user_name query params.
	if dbName := r.URL.Query().Get("database_name"); dbName != "" {
		filtered := make([]map[string]any, 0)
		for _, p := range result {
			if p["database_name"] == dbName {
				filtered = append(filtered, p)
			}
		}
		result = filtered
	}
	if userName := r.URL.Query().Get("user_name"); userName != "" {
		filtered := make([]map[string]any, 0)
		for _, p := range result {
			if p["user_name"] == userName {
				filtered = append(filtered, p)
			}
		}
		result = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"privileges": result, "total_count": len(result)})
}

func (app *Application) SetRDBSettings(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	settings, _ := body["settings"].([]any)
	if settings == nil {
		settings = []any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

// --- RDB Read Replicas ---

func (app *Application) CreateRDBReadReplica(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateRDBReadReplica(chi.URLParam(r, "region"), chi.URLParam(r, "instance_id"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetRDBReadReplica(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetRDBReadReplica(chi.URLParam(r, "read_replica_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteRDBReadReplica(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.DeleteRDBReadReplica(chi.URLParam(r, "read_replica_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) CreateRDBReadReplicaEndpoint(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateRDBReadReplicaEndpoint(chi.URLParam(r, "read_replica_id"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) PromoteRDBReadReplica(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.PromoteRDBReadReplica(chi.URLParam(r, "read_replica_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ResetRDBReadReplica(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.ResetRDBReadReplica(chi.URLParam(r, "read_replica_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// --- RDB Snapshots ---

func (app *Application) CreateRDBSnapshot(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateRDBSnapshot(chi.URLParam(r, "region"), chi.URLParam(r, "instance_id"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetRDBSnapshot(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetRDBSnapshot(chi.URLParam(r, "snapshot_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListRDBSnapshots(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListRDBSnapshots(chi.URLParam(r, "region"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "snapshots", items)
}

func (app *Application) UpdateRDBSnapshot(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateRDBSnapshot(chi.URLParam(r, "snapshot_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteRDBSnapshot(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.DeleteRDBSnapshot(chi.URLParam(r, "snapshot_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) CreateRDBInstanceFromSnapshot(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateRDBInstanceFromSnapshot(chi.URLParam(r, "region"), chi.URLParam(r, "snapshot_id"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// --- RDB Backups ---

func (app *Application) CreateRDBBackup(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	instanceID := r.URL.Query().Get("instance_id")
	if instanceID == "" {
		instanceID, _ = body["instance_id"].(string)
	}
	out, err := app.repo.CreateRDBBackup(chi.URLParam(r, "region"), instanceID, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetRDBBackup(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetRDBBackup(chi.URLParam(r, "backup_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListRDBBackups(w http.ResponseWriter, r *http.Request) {
	instanceID := r.URL.Query().Get("instance_id")
	items, err := app.repo.ListRDBBackups(chi.URLParam(r, "region"), instanceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "database_backups", items)
}

func (app *Application) UpdateRDBBackup(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateRDBBackup(chi.URLParam(r, "backup_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteRDBBackup(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.DeleteRDBBackup(chi.URLParam(r, "backup_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ExportRDBBackup(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.ExportRDBBackup(chi.URLParam(r, "backup_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) RestoreRDBBackup(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	instanceID, _ := body["instance_id"].(string)
	out, err := app.repo.RestoreRDBBackup(instanceID, chi.URLParam(r, "backup_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// --- RDB Certificate, Logs, Endpoints ---

func (app *Application) RenewRDBCertificate(w http.ResponseWriter, r *http.Request) {
	if _, err := app.repo.GetRDBInstance(chi.URLParam(r, "instance_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	pem := "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALHMPMCJ+OebMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBm1v\nY2t3YTAeFw0yNDAyMjQwMDAwMDBaFw0zNDAyMjQwMDAwMDBaMBExDzANBgNVBAMM\nBm1vY2t3YTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o35FHQOGT7Pmb+oCaFHh\nOBAAPHlNmjNKHEl2hdNRMNwIDAQABMA0GCSqGSIb3DQEBCwUAA0EA\n-----END CERTIFICATE-----\n"
	writeJSON(w, http.StatusOK, map[string]any{
		"certificate": map[string]any{
			"content": base64.StdEncoding.EncodeToString([]byte(pem)),
		},
	})
}

func (app *Application) PrepareRDBInstanceLogs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"logs": []any{}, "total_count": 0})
}

func (app *Application) CreateRDBEndpoint(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateRDBEndpoint(chi.URLParam(r, "instance_id"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteRDBEndpoint(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteRDBEndpoint(chi.URLParam(r, "endpoint_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (app *Application) ListRDBNodeTypes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"node_types": []any{
			map[string]any{"name": "DB-DEV-S", "stock_status": "available", "memory": float64(1000000000), "vcpus": float64(2)},
			map[string]any{"name": "DB-DEV-M", "stock_status": "available", "memory": float64(2000000000), "vcpus": float64(2)},
			map[string]any{"name": "DB-DEV-L", "stock_status": "available", "memory": float64(4000000000), "vcpus": float64(4)},
			map[string]any{"name": "DB-GP-XS", "stock_status": "available", "memory": float64(8000000000), "vcpus": float64(4)},
		},
		"total_count": 4,
	})
}

// CreateRDBReadReplicaTopLevel handles POST /rdb/v1/regions/{region}/read-replicas
// where instance_id is provided in the request body (SDK top-level path).
func (app *Application) CreateRDBReadReplicaTopLevel(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	instanceID, _ := body["instance_id"].(string)
	out, err := app.repo.CreateRDBReadReplica(chi.URLParam(r, "region"), instanceID, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
