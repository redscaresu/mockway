package handlers

import (
	"encoding/json"
	"errors"
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
	r.Get("/mock/state", app.GetState)
	r.Get("/mock/state/{service}", app.GetServiceState)

	r.Group(func(r chi.Router) {
		r.Use(app.requireAuthToken)

		r.Route("/instance/v1/zones/{zone}", func(r chi.Router) {
			r.Post("/servers", app.CreateServer)
			r.Get("/servers", app.ListServers)
			r.Get("/servers/{server_id}", app.GetServer)
			r.Delete("/servers/{server_id}", app.DeleteServer)

			r.Post("/ips", app.CreateIP)
			r.Get("/ips", app.ListIPs)
			r.Get("/ips/{ip_id}", app.GetIP)
			r.Delete("/ips/{ip_id}", app.DeleteIP)

			r.Post("/security_groups", app.CreateSecurityGroup)
			r.Get("/security_groups", app.ListSecurityGroups)
			r.Get("/security_groups/{sg_id}", app.GetSecurityGroup)
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
			r.Delete("/vpcs/{vpc_id}", app.DeleteVPC)

			r.Post("/private-networks", app.CreatePrivateNetwork)
			r.Get("/private-networks", app.ListPrivateNetworks)
			r.Get("/private-networks/{pn_id}", app.GetPrivateNetwork)
			r.Delete("/private-networks/{pn_id}", app.DeletePrivateNetwork)
		})

		r.Route("/lb/v1/zones/{zone}", func(r chi.Router) {
			r.Post("/lbs", app.CreateLB)
			r.Get("/lbs", app.ListLBs)
			r.Get("/lbs/{lb_id}", app.GetLB)
			r.Delete("/lbs/{lb_id}", app.DeleteLB)

			r.Post("/frontends", app.CreateFrontend)
			r.Get("/frontends", app.ListFrontends)
			r.Get("/frontends/{frontend_id}", app.GetFrontend)
			r.Delete("/frontends/{frontend_id}", app.DeleteFrontend)

			r.Post("/backends", app.CreateBackend)
			r.Get("/backends", app.ListBackends)
			r.Get("/backends/{backend_id}", app.GetBackend)
			r.Delete("/backends/{backend_id}", app.DeleteBackend)

			r.Post("/lbs/{lb_id}/private-networks", app.AttachLBPrivateNetwork)
			r.Get("/lbs/{lb_id}/private-networks", app.ListLBPrivateNetworks)
			r.Delete("/lbs/{lb_id}/private-networks/{pn_id}", app.DeleteLBPrivateNetwork)
		})

		r.Route("/k8s/v1/regions/{region}", func(r chi.Router) {
			r.Post("/clusters", app.CreateCluster)
			r.Get("/clusters", app.ListClusters)
			r.Get("/clusters/{cluster_id}", app.GetCluster)
			r.Delete("/clusters/{cluster_id}", app.DeleteCluster)

			r.Post("/clusters/{cluster_id}/pools", app.CreatePool)
			r.Get("/clusters/{cluster_id}/pools", app.ListPools)
			r.Get("/pools/{pool_id}", app.GetPool)
			r.Delete("/pools/{pool_id}", app.DeletePool)
		})

		r.Route("/rdb/v1/regions/{region}", func(r chi.Router) {
			r.Post("/instances", app.CreateRDBInstance)
			r.Get("/instances", app.ListRDBInstances)
			r.Get("/instances/{instance_id}", app.GetRDBInstance)
			r.Delete("/instances/{instance_id}", app.DeleteRDBInstance)

			r.Post("/instances/{instance_id}/databases", app.CreateRDBDatabase)
			r.Get("/instances/{instance_id}/databases", app.ListRDBDatabases)
			r.Delete("/instances/{instance_id}/databases/{db_name}", app.DeleteRDBDatabase)

			r.Post("/instances/{instance_id}/users", app.CreateRDBUser)
			r.Get("/instances/{instance_id}/users", app.ListRDBUsers)
			r.Delete("/instances/{instance_id}/users/{user_name}", app.DeleteRDBUser)
		})

		r.Route("/iam/v1alpha1", func(r chi.Router) {
			r.Post("/applications", app.CreateIAMApplication)
			r.Get("/applications", app.ListIAMApplications)
			r.Get("/applications/{application_id}", app.GetIAMApplication)
			r.Delete("/applications/{application_id}", app.DeleteIAMApplication)

			r.Post("/api-keys", app.CreateIAMAPIKey)
			r.Get("/api-keys", app.ListIAMAPIKeys)
			r.Get("/api-keys/{access_key}", app.GetIAMAPIKey)
			r.Delete("/api-keys/{access_key}", app.DeleteIAMAPIKey)

			r.Post("/policies", app.CreateIAMPolicy)
			r.Get("/policies", app.ListIAMPolicies)
			r.Get("/policies/{policy_id}", app.GetIAMPolicy)
			r.Delete("/policies/{policy_id}", app.DeleteIAMPolicy)

			r.Post("/ssh-keys", app.CreateIAMSSHKey)
			r.Get("/ssh-keys", app.ListIAMSSHKeys)
			r.Get("/ssh-keys/{ssh_key_id}", app.GetIAMSSHKey)
			r.Delete("/ssh-keys/{ssh_key_id}", app.DeleteIAMSSHKey)
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
