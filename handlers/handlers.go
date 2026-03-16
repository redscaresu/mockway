package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/redscaresu/mockway/models"
	"github.com/redscaresu/mockway/repository"
)

type Application struct {
	repo *repository.Repository
}

func NewApplication(repo *repository.Repository) *Application {
	return &Application{repo: repo}
}

func (app *Application) RegisterRoutes(r chi.Router) {
	// Admin routes do not require auth.
	r.Post("/mock/reset", app.ResetState)
	r.Post("/mock/snapshot", app.SnapshotState)
	r.Post("/mock/restore", app.RestoreState)
	r.Get("/mock/state", app.GetState)
	r.Get("/mock/state/{service}", app.GetServiceState)

	r.Group(func(r chi.Router) {
		r.Use(app.requireAuthToken)

		r.Route("/marketplace/v2", func(r chi.Router) {
			r.Get("/local-images", app.ListMarketplaceLocalImages)
			r.Get("/local-images/{local_image_id}", app.GetMarketplaceLocalImage)
		})

		r.Route("/instance/v1/zones/{zone}", func(r chi.Router) {
			r.Get("/products/servers", app.ListProductsServers)

			r.Post("/servers", app.CreateServer)
			r.Get("/servers", app.ListServers)
			r.Get("/servers/{server_id}", app.GetServer)
			r.Patch("/servers/{server_id}", app.UpdateServer)
			r.Post("/servers/{server_id}/action", app.ServerAction)
			r.Post("/volumes", app.CreateVolume)
			r.Get("/volumes", app.ListVolumes)
			r.Get("/volumes/{volume_id}", app.GetVolume)
			r.Patch("/volumes/{volume_id}", app.PatchVolume)
			r.Delete("/volumes/{volume_id}", app.DeleteVolume)
			r.Get("/servers/{server_id}/user_data", app.ListServerUserData)
			r.Get("/servers/{server_id}/user_data/{key}", app.GetServerUserDataKey)
			r.Patch("/servers/{server_id}/user_data/{key}", app.SetServerUserData)
			r.Delete("/servers/{server_id}", app.DeleteServer)

			r.Post("/ips", app.CreateIP)
			r.Get("/ips", app.ListIPs)
			r.Get("/ips/{ip_id}", app.GetIP)
			r.Patch("/ips/{ip_id}", app.UpdateInstanceIP)
			r.Delete("/ips/{ip_id}", app.DeleteIP)

			r.Post("/security_groups", app.CreateSecurityGroup)
			r.Get("/security_groups", app.ListSecurityGroups)
			r.Get("/security_groups/{sg_id}", app.GetSecurityGroup)
			r.Patch("/security_groups/{sg_id}", app.UpdateSecurityGroup)
			r.Put("/security_groups/{sg_id}/rules", app.SetSecurityGroupRules)
			r.Get("/security_groups/{sg_id}/rules", app.GetSecurityGroupRules)
			r.Delete("/security_groups/{sg_id}", app.DeleteSecurityGroup)

			r.Post("/servers/{server_id}/private_nics", app.CreatePrivateNIC)
			r.Get("/servers/{server_id}/private_nics", app.ListPrivateNICs)
			r.Get("/servers/{server_id}/private_nics/{nic_id}", app.GetPrivateNIC)
			r.Delete("/servers/{server_id}/private_nics/{nic_id}", app.DeletePrivateNIC)
		})

		r.Route("/vpc/v1/regions/{region}", func(r chi.Router) {
			r.Post("/vpcs", app.CreateVPC)
			r.Get("/vpcs", app.ListVPCs)
			r.Get("/vpcs/{vpc_id}", app.GetVPC)
			r.Patch("/vpcs/{vpc_id}", app.UpdateVPC)
			r.Delete("/vpcs/{vpc_id}", app.DeleteVPC)

			r.Post("/private-networks", app.CreatePrivateNetwork)
			r.Get("/private-networks", app.ListPrivateNetworks)
			r.Get("/private-networks/{pn_id}", app.GetPrivateNetwork)
			r.Patch("/private-networks/{pn_id}", app.UpdatePrivateNetwork)
			r.Delete("/private-networks/{pn_id}", app.DeletePrivateNetwork)
		})

		r.Route("/vpc/v2/regions/{region}", func(r chi.Router) {
			r.Post("/vpcs", app.CreateVPC)
			r.Get("/vpcs", app.ListVPCs)
			r.Get("/vpcs/{vpc_id}", app.GetVPC)
			r.Patch("/vpcs/{vpc_id}", app.UpdateVPC)
			r.Delete("/vpcs/{vpc_id}", app.DeleteVPC)

			r.Post("/private-networks", app.CreatePrivateNetwork)
			r.Get("/private-networks", app.ListPrivateNetworks)
			r.Get("/private-networks/{pn_id}", app.GetPrivateNetwork)
			r.Patch("/private-networks/{pn_id}", app.UpdatePrivateNetwork)
			r.Delete("/private-networks/{pn_id}", app.DeletePrivateNetwork)
		})

		r.Route("/lb/v1/zones/{zone}", func(r chi.Router) {
			r.Post("/ips", app.CreateLBIP)
			r.Get("/ips", app.ListLBIPs)
			r.Get("/ips/{ip_id}", app.GetLBIP)
			r.Patch("/ips/{ip_id}", app.UpdateLBIP)
			r.Delete("/ips/{ip_id}", app.DeleteLBIP)

			r.Post("/lbs", app.CreateLB)
			r.Get("/lbs", app.ListLBs)
			r.Get("/lbs/{lb_id}", app.GetLB)
			r.Put("/lbs/{lb_id}", app.UpdateLB)
			r.Delete("/lbs/{lb_id}", app.DeleteLB)

			r.Post("/frontends", app.CreateFrontend)
			r.Get("/frontends", app.ListFrontends)
			r.Get("/frontends/{frontend_id}", app.GetFrontend)
			r.Put("/frontends/{frontend_id}", app.UpdateFrontend)
			r.Get("/frontends/{frontend_id}/acls", app.ListFrontendACLs)
			r.Delete("/frontends/{frontend_id}", app.DeleteFrontend)

			r.Post("/backends", app.CreateBackend)
			r.Get("/backends", app.ListBackends)
			r.Get("/backends/{backend_id}", app.GetBackend)
			r.Put("/backends/{backend_id}", app.UpdateBackend)
			r.Delete("/backends/{backend_id}", app.DeleteBackend)

			r.Post("/lbs/{lb_id}/backends", app.CreateBackend)
			r.Get("/lbs/{lb_id}/backends", app.ListBackends)
			r.Post("/lbs/{lb_id}/frontends", app.CreateFrontend)
			r.Get("/lbs/{lb_id}/frontends", app.ListFrontends)

			r.Post("/lbs/{lb_id}/private-networks", app.AttachLBPrivateNetwork)
			r.Post("/lbs/{lb_id}/attach-private-network", app.AttachLBPrivateNetwork)
			r.Get("/lbs/{lb_id}/private-networks", app.ListLBPrivateNetworks)
			r.Delete("/lbs/{lb_id}/private-networks/{pn_id}", app.DeleteLBPrivateNetwork)
			r.Post("/lbs/{lb_id}/detach-private-network", app.DetachLBPrivateNetwork)

			r.Post("/frontends/{frontend_id}/acls", app.CreateLBACL)
			r.Get("/acls/{acl_id}", app.GetLBACL)
			r.Put("/acls/{acl_id}", app.UpdateLBACL)
			r.Delete("/acls/{acl_id}", app.DeleteLBACL)

			r.Post("/routes", app.CreateLBRoute)
			r.Get("/routes", app.ListLBRoutes)
			r.Get("/routes/{route_id}", app.GetLBRoute)
			r.Put("/routes/{route_id}", app.UpdateLBRoute)
			r.Delete("/routes/{route_id}", app.DeleteLBRoute)

			r.Post("/lbs/{lb_id}/certificates", app.CreateLBCertificate)
			r.Get("/lbs/{lb_id}/certificates", app.ListLBCertificates)
			r.Get("/certificates/{certificate_id}", app.GetLBCertificate)
			r.Put("/certificates/{certificate_id}", app.UpdateLBCertificate)
			r.Delete("/certificates/{certificate_id}", app.DeleteLBCertificate)

			r.Put("/backends/{backend_id}/healthcheck", app.UpdateLBHealthCheck)
			r.Put("/backends/{backend_id}/servers", app.SetLBBackendServers)
			r.Post("/lbs/{lb_id}/migrate", app.MigrateLB)
		})

		r.Route("/lb/v1/regions/{region}", func(r chi.Router) {
			r.Post("/ips", app.CreateLBIP)
			r.Get("/ips", app.ListLBIPs)
			r.Get("/ips/{ip_id}", app.GetLBIP)
			r.Patch("/ips/{ip_id}", app.UpdateLBIP)
			r.Delete("/ips/{ip_id}", app.DeleteLBIP)

			r.Post("/lbs", app.CreateLB)
			r.Get("/lbs", app.ListLBs)
			r.Get("/lbs/{lb_id}", app.GetLB)
			r.Put("/lbs/{lb_id}", app.UpdateLB)
			r.Delete("/lbs/{lb_id}", app.DeleteLB)

			r.Post("/frontends", app.CreateFrontend)
			r.Get("/frontends", app.ListFrontends)
			r.Get("/frontends/{frontend_id}", app.GetFrontend)
			r.Put("/frontends/{frontend_id}", app.UpdateFrontend)
			r.Get("/frontends/{frontend_id}/acls", app.ListFrontendACLs)
			r.Delete("/frontends/{frontend_id}", app.DeleteFrontend)

			r.Post("/backends", app.CreateBackend)
			r.Get("/backends", app.ListBackends)
			r.Get("/backends/{backend_id}", app.GetBackend)
			r.Put("/backends/{backend_id}", app.UpdateBackend)
			r.Delete("/backends/{backend_id}", app.DeleteBackend)

			r.Post("/lbs/{lb_id}/backends", app.CreateBackend)
			r.Get("/lbs/{lb_id}/backends", app.ListBackends)
			r.Post("/lbs/{lb_id}/frontends", app.CreateFrontend)
			r.Get("/lbs/{lb_id}/frontends", app.ListFrontends)

			r.Post("/lbs/{lb_id}/private-networks", app.AttachLBPrivateNetwork)
			r.Post("/lbs/{lb_id}/attach-private-network", app.AttachLBPrivateNetwork)
			r.Get("/lbs/{lb_id}/private-networks", app.ListLBPrivateNetworks)
			r.Delete("/lbs/{lb_id}/private-networks/{pn_id}", app.DeleteLBPrivateNetwork)
			r.Post("/lbs/{lb_id}/detach-private-network", app.DetachLBPrivateNetwork)
			r.Post("/lbs/{lb_id}/private-networks/{pn_id}/attach", app.AttachLBPrivateNetwork)
			r.Post("/lbs/{lb_id}/private-networks/{pn_id}/detach", app.DetachLBPrivateNetwork)

			r.Post("/frontends/{frontend_id}/acls", app.CreateLBACL)
			r.Get("/frontends/{frontend_id}/acls", app.ListFrontendACLs)
			r.Get("/acls/{acl_id}", app.GetLBACL)
			r.Put("/acls/{acl_id}", app.UpdateLBACL)
			r.Delete("/acls/{acl_id}", app.DeleteLBACL)

			r.Post("/routes", app.CreateLBRoute)
			r.Get("/routes", app.ListLBRoutes)
			r.Get("/routes/{route_id}", app.GetLBRoute)
			r.Put("/routes/{route_id}", app.UpdateLBRoute)
			r.Delete("/routes/{route_id}", app.DeleteLBRoute)

			r.Post("/lbs/{lb_id}/certificates", app.CreateLBCertificate)
			r.Get("/lbs/{lb_id}/certificates", app.ListLBCertificates)
			r.Get("/certificates/{certificate_id}", app.GetLBCertificate)
			r.Put("/certificates/{certificate_id}", app.UpdateLBCertificate)
			r.Delete("/certificates/{certificate_id}", app.DeleteLBCertificate)

			r.Put("/backends/{backend_id}/healthcheck", app.UpdateLBHealthCheck)
			r.Put("/backends/{backend_id}/servers", app.SetLBBackendServers)
			r.Post("/lbs/{lb_id}/migrate", app.MigrateLB)
		})

		r.Route("/k8s/v1/regions/{region}", func(r chi.Router) {
			r.Get("/versions", app.ListK8sVersions)
			r.Get("/versions/{version_name}", app.GetK8sVersion)

			r.Post("/clusters", app.CreateCluster)
			r.Get("/clusters", app.ListClusters)
			r.Get("/clusters/{cluster_id}", app.GetCluster)
			r.Get("/clusters/{cluster_id}/kubeconfig", app.GetClusterKubeconfig)
			r.Get("/clusters/{cluster_id}/nodes", app.ListClusterNodes)
			r.Patch("/clusters/{cluster_id}", app.UpdateCluster)
			r.Post("/clusters/{cluster_id}/upgrade", app.UpgradeCluster)
			r.Post("/clusters/{cluster_id}/set-type", app.SetClusterType)
			r.Delete("/clusters/{cluster_id}", app.DeleteCluster)

			r.Post("/clusters/{cluster_id}/pools", app.CreatePool)
			r.Get("/clusters/{cluster_id}/pools", app.ListPools)
			r.Get("/pools/{pool_id}", app.GetPool)
			r.Patch("/pools/{pool_id}", app.UpdatePool)
			r.Post("/pools/{pool_id}/upgrade", app.UpgradePool)
			r.Delete("/pools/{pool_id}", app.DeletePool)

			r.Get("/nodes/{node_id}", app.GetNode)
		})

		r.Route("/rdb/v1/regions/{region}", func(r chi.Router) {
			r.Get("/node-types", app.ListRDBNodeTypes)

			r.Post("/instances", app.CreateRDBInstance)
			r.Get("/instances", app.ListRDBInstances)
			r.Get("/instances/{instance_id}", app.GetRDBInstance)
			r.Patch("/instances/{instance_id}", app.UpdateRDBInstance)
			r.Post("/instances/{instance_id}/upgrade", app.UpgradeRDBInstance)
			r.Get("/instances/{instance_id}/certificate", app.GetRDBCertificate)
			r.Delete("/instances/{instance_id}", app.DeleteRDBInstance)

			r.Post("/instances/{instance_id}/databases", app.CreateRDBDatabase)
			r.Get("/instances/{instance_id}/databases", app.ListRDBDatabases)
			r.Delete("/instances/{instance_id}/databases/{db_name}", app.DeleteRDBDatabase)

			r.Post("/instances/{instance_id}/users", app.CreateRDBUser)
			r.Get("/instances/{instance_id}/users", app.ListRDBUsers)
			r.Patch("/instances/{instance_id}/users/{user_name}", app.UpdateRDBUser)
			r.Delete("/instances/{instance_id}/users/{user_name}", app.DeleteRDBUser)

			r.Put("/instances/{instance_id}/acls", app.SetRDBACLs)
			r.Get("/instances/{instance_id}/acls", app.ListRDBACLs)
			r.Delete("/instances/{instance_id}/acls", app.DeleteRDBACLs)
			r.Put("/instances/{instance_id}/privileges", app.SetRDBPrivileges)
			r.Get("/instances/{instance_id}/privileges", app.ListRDBPrivileges)
			r.Put("/instances/{instance_id}/settings", app.SetRDBSettings)

			r.Post("/read-replicas", app.CreateRDBReadReplicaTopLevel)
			r.Post("/instances/{instance_id}/read-replicas", app.CreateRDBReadReplica)
			r.Get("/read-replicas/{read_replica_id}", app.GetRDBReadReplica)
			r.Delete("/read-replicas/{read_replica_id}", app.DeleteRDBReadReplica)
			r.Post("/read-replicas/{read_replica_id}/endpoints", app.CreateRDBReadReplicaEndpoint)
			r.Post("/read-replicas/{read_replica_id}/promote", app.PromoteRDBReadReplica)
			r.Post("/read-replicas/{read_replica_id}/reset", app.ResetRDBReadReplica)

			r.Post("/instances/{instance_id}/snapshots", app.CreateRDBSnapshot)
			r.Get("/snapshots", app.ListRDBSnapshots)
			r.Get("/snapshots/{snapshot_id}", app.GetRDBSnapshot)
			r.Patch("/snapshots/{snapshot_id}", app.UpdateRDBSnapshot)
			r.Delete("/snapshots/{snapshot_id}", app.DeleteRDBSnapshot)
			r.Post("/snapshots/{snapshot_id}/create-instance", app.CreateRDBInstanceFromSnapshot)

			r.Post("/backups", app.CreateRDBBackup)
			r.Get("/backups", app.ListRDBBackups)
			r.Get("/backups/{backup_id}", app.GetRDBBackup)
			r.Patch("/backups/{backup_id}", app.UpdateRDBBackup)
			r.Delete("/backups/{backup_id}", app.DeleteRDBBackup)
			r.Post("/backups/{backup_id}/export", app.ExportRDBBackup)
			r.Post("/backups/{backup_id}/restore", app.RestoreRDBBackup)

			r.Post("/instances/{instance_id}/renew-certificate", app.RenewRDBCertificate)
			r.Post("/instances/{instance_id}/prepare-logs", app.PrepareRDBInstanceLogs)
			r.Post("/instances/{instance_id}/endpoints", app.CreateRDBEndpoint)
			r.Delete("/endpoints/{endpoint_id}", app.DeleteRDBEndpoint)
		})

		r.Route("/redis/v1/zones/{zone}", func(r chi.Router) {
			r.Post("/clusters", app.CreateRedisCluster)
			r.Get("/clusters", app.ListRedisClusters)
			r.Get("/clusters/{cluster_id}", app.GetRedisCluster)
			r.Get("/clusters/{cluster_id}/certificate", app.GetRedisCertificate)
			r.Patch("/clusters/{cluster_id}", app.UpdateRedisCluster)
			r.Delete("/clusters/{cluster_id}", app.DeleteRedisCluster)
			r.Post("/clusters/{cluster_id}/migrate", app.MigrateRedisCluster)
			r.Put("/clusters/{cluster_id}/acls", app.SetRedisACLRules)
			r.Put("/clusters/{cluster_id}/endpoints", app.SetRedisEndpoints)
			r.Put("/clusters/{cluster_id}/settings", app.SetRedisClusterSettings)
			r.Delete("/endpoints/{endpoint_id}", app.DeleteRedisEndpoint)
			r.Get("/cluster-versions", app.ListRedisClusterVersions)
			r.Get("/node-types", app.ListRedisNodeTypes)
		})

		r.Route("/registry/v1/regions/{region}", func(r chi.Router) {
			r.Post("/namespaces", app.CreateRegistryNamespace)
			r.Get("/namespaces", app.ListRegistryNamespaces)
			r.Get("/namespaces/{namespace_id}", app.GetRegistryNamespace)
			r.Patch("/namespaces/{namespace_id}", app.UpdateRegistryNamespace)
			r.Delete("/namespaces/{namespace_id}", app.DeleteRegistryNamespace)
		})

		r.Route("/iam/v1alpha1", func(r chi.Router) {
			r.Post("/applications", app.CreateIAMApplication)
			r.Get("/applications", app.ListIAMApplications)
			r.Get("/applications/{application_id}", app.GetIAMApplication)
			r.Patch("/applications/{application_id}", app.UpdateIAMApplication)
			r.Delete("/applications/{application_id}", app.DeleteIAMApplication)

			r.Post("/api-keys", app.CreateIAMAPIKey)
			r.Get("/api-keys", app.ListIAMAPIKeys)
			r.Get("/api-keys/{access_key}", app.GetIAMAPIKey)
			r.Patch("/api-keys/{access_key}", app.UpdateIAMAPIKey)
			r.Delete("/api-keys/{access_key}", app.DeleteIAMAPIKey)

			r.Post("/policies", app.CreateIAMPolicy)
			r.Get("/policies", app.ListIAMPolicies)
			r.Get("/policies/{policy_id}", app.GetIAMPolicy)
			r.Patch("/policies/{policy_id}", app.UpdateIAMPolicy)
			r.Delete("/policies/{policy_id}", app.DeleteIAMPolicy)
			r.Post("/rules", app.CreateIAMRule)
			r.Get("/rules", app.ListIAMRules)
			r.Put("/rules", app.SetIAMRules)

			r.Post("/ssh-keys", app.CreateIAMSSHKey)
			r.Get("/ssh-keys", app.ListIAMSSHKeys)
			r.Get("/ssh-keys/{ssh_key_id}", app.GetIAMSSHKey)
			r.Patch("/ssh-keys/{ssh_key_id}", app.UpdateIAMSSHKey)
			r.Delete("/ssh-keys/{ssh_key_id}", app.DeleteIAMSSHKey)

			r.Post("/users", app.CreateIAMUser)
			r.Get("/users", app.ListIAMUsers)
			r.Get("/users/{user_id}", app.GetIAMUser)
			r.Patch("/users/{user_id}", app.UpdateIAMUser)
			r.Delete("/users/{user_id}", app.DeleteIAMUser)
			r.Post("/users/{user_id}/update-username", app.UpdateIAMUserUsername)

			r.Post("/groups", app.CreateIAMGroup)
			r.Get("/groups", app.ListIAMGroups)
			r.Get("/groups/{group_id}", app.GetIAMGroup)
			r.Patch("/groups/{group_id}", app.UpdateIAMGroup)
			r.Delete("/groups/{group_id}", app.DeleteIAMGroup)
			r.Post("/groups/{group_id}/add-member", app.AddIAMGroupMember)
			r.Post("/groups/{group_id}/remove-member", app.RemoveIAMGroupMember)
			r.Put("/groups/{group_id}/members", app.SetIAMGroupMembers)
		})

		r.Route("/block/v1alpha1/zones/{zone}", func(r chi.Router) {
			r.Post("/volumes", app.CreateBlockVolumeHandler)
			r.Get("/volumes", app.ListBlockVolumes)
			r.Get("/volumes/{volume_id}", app.GetBlockVolumeHandler)
			r.Patch("/volumes/{volume_id}", app.UpdateBlockVolumeHandler)
			r.Delete("/volumes/{volume_id}", app.DeleteBlockVolumeHandler)
			r.Post("/snapshots", app.CreateBlockSnapshotHandler)
			r.Get("/snapshots", app.ListBlockSnapshots)
			r.Get("/snapshots/{snapshot_id}", app.GetBlockSnapshotHandler)
			r.Patch("/snapshots/{snapshot_id}", app.UpdateBlockSnapshotHandler)
			r.Delete("/snapshots/{snapshot_id}", app.DeleteBlockSnapshotHandler)
			r.Get("/volume-types", app.ListBlockVolumeTypes)
		})

		r.Route("/ipam/v1/regions/{region}", func(r chi.Router) {
			r.Get("/ips", app.ListIPAMIPs)
			r.Post("/ips", app.CreateIPAMIP)
			r.Get("/ips/{ip_id}", app.GetIPAMIP)
			r.Patch("/ips/{ip_id}", app.UpdateIPAMIP)
			r.Delete("/ips/{ip_id}", app.DeleteIPAMIP)
			r.Post("/ips/{ip_id}/detach", app.DetachIPAMIP)
			r.Post("/ips/{ip_id}/move", app.MoveIPAMIP)
		})

		r.Get("/domain/v2beta1/dns-zones", app.ListDNSZones)
		r.Route("/domain/v2beta1/dns-zones/{dns_zone}", func(r chi.Router) {
			r.Patch("/records", app.PatchDomainRecords)
			r.Get("/records", app.ListDomainRecords)
		})

		// Legacy alias for scaleway_account_ssh_key.
		r.Route("/account/v2alpha1", func(r chi.Router) {
			r.Post("/ssh-keys", app.CreateIAMSSHKey)
			r.Get("/ssh-keys", app.ListIAMSSHKeys)
			r.Get("/ssh-keys/{ssh_key_id}", app.GetIAMSSHKey)
			r.Delete("/ssh-keys/{ssh_key_id}", app.DeleteIAMSSHKey)
		})
	})
}

func (app *Application) requireAuthToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth-Token") == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"message": "missing or empty X-Auth-Token",
				"type":    "denied_authentication",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decodeBody(r *http.Request) (map[string]any, error) {
	defer r.Body.Close()
	if r.Body == nil {
		return map[string]any{}, nil
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if body == nil {
		body = map[string]any{}
	}
	return body, nil
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func writeList(w http.ResponseWriter, key string, items []map[string]any) {
	writeJSON(w, http.StatusOK, map[string]any{
		key:           items,
		"total_count": len(items),
	})
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, models.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "resource not found", "type": "not_found"})
	case errors.Is(err, models.ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]any{"message": "cannot delete: dependents exist", "type": "conflict"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "internal server error", "type": "internal"})
	}
}

func writeCreateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, models.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "referenced resource not found", "type": "not_found"})
	case errors.Is(err, models.ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]any{"message": "resource already exists", "type": "conflict"})
	default:
		writeDomainError(w, err)
	}
}
