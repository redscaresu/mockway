package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (app *Application) ListK8sVersions(w http.ResponseWriter, r *http.Request) {
	versions := []map[string]any{
		{
			"name":                    "1.31.2",
			"label":                   "Kubernetes 1.31.2",
			"available_cnis":          []any{"cilium", "calico", "kilo", "flannel"},
			"available_container_runtimes": []any{"containerd"},
			"available_feature_gates": []any{},
			"available_kubelet_args":  map[string]any{},
		},
		{
			"name":                    "1.30.6",
			"label":                   "Kubernetes 1.30.6",
			"available_cnis":          []any{"cilium", "calico", "kilo", "flannel"},
			"available_container_runtimes": []any{"containerd"},
			"available_feature_gates": []any{},
			"available_kubelet_args":  map[string]any{},
		},
		{
			"name":                    "1.29.10",
			"label":                   "Kubernetes 1.29.10",
			"available_cnis":          []any{"cilium", "calico", "kilo", "flannel"},
			"available_container_runtimes": []any{"containerd"},
			"available_feature_gates": []any{},
			"available_kubelet_args":  map[string]any{},
		},
		{
			"name":                    "1.28.15",
			"label":                   "Kubernetes 1.28.15",
			"available_cnis":          []any{"cilium", "calico", "kilo", "flannel"},
			"available_container_runtimes": []any{"containerd"},
			"available_feature_gates": []any{},
			"available_kubelet_args":  map[string]any{},
		},
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

func (app *Application) ListClusterNodes(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "cluster_id")
	// Return nodes based on existing pools for this cluster.
	pools, err := app.repo.ListPoolsByCluster(clusterID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	nodes := []map[string]any{}
	for _, pool := range pools {
		poolID, _ := pool["id"].(string)
		poolName, _ := pool["name"].(string)
		size, _ := pool["size"].(float64)
		if size < 1 {
			size = 1
		}
		for i := 0; i < int(size); i++ {
			nodes = append(nodes, map[string]any{
				"id":         uuid.NewString(),
				"pool_id":    poolID,
				"cluster_id": clusterID,
				"region":     chi.URLParam(r, "region"),
				"name":       fmt.Sprintf("%s-node-%d", poolName, i),
				"status":     "ready",
				"public_ip_v4": nil,
				"public_ip_v6": nil,
				"conditions":   map[string]any{},
				"created_at":   pool["created_at"],
				"updated_at":   pool["updated_at"],
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"nodes":       nodes,
		"total_count": len(nodes),
	})
}

func (app *Application) GetClusterKubeconfig(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "cluster_id")
	if _, err := app.repo.GetCluster(clusterID); err != nil {
		writeDomainError(w, err)
		return
	}
	// Return a minimal mock kubeconfig the provider can parse.
	kubeconfig := map[string]any{
		"name":            "kubeconfig",
		"content_type":    "application/octet-stream",
		"content":         "YXBpVmVyc2lvbjogdjEKY2x1c3RlcnM6Ci0gY2x1c3RlcjoKICAgIHNlcnZlcjogaHR0cHM6Ly9tb2NrLWs4cy1hcGlzZXJ2ZXIuc2N3LmNsb3VkOjY0NDMKICBuYW1lOiBtb2NrCmNvbnRleHRzOgotIGNvbnRleHQ6CiAgICBjbHVzdGVyOiBtb2NrCiAgICB1c2VyOiBtb2NrCiAgbmFtZTogbW9jawpjdXJyZW50LWNvbnRleHQ6IG1vY2sKa2luZDogQ29uZmlnCnVzZXJzOgotIG5hbWU6IG1vY2sKICB1c2VyOgogICAgdG9rZW46IG1vY2stdG9rZW4K",
	}
	writeJSON(w, http.StatusOK, kubeconfig)
}

func (app *Application) CreateCluster(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateCluster(chi.URLParam(r, "region"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetCluster(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetCluster(chi.URLParam(r, "cluster_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListClusters(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListClusters(chi.URLParam(r, "region"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "clusters", items)
}

func (app *Application) UpdateCluster(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateCluster(chi.URLParam(r, "cluster_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "cluster_id")
	// The Scaleway SDK expects DELETE to return the cluster object so it can
	// poll for deletion completion using the cluster ID from the response.
	out, err := app.repo.GetCluster(clusterID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if err := app.repo.DeleteCluster(clusterID); err != nil {
		writeDomainError(w, err)
		return
	}
	out["status"] = "deleting"
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) CreatePool(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreatePool(chi.URLParam(r, "region"), chi.URLParam(r, "cluster_id"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetPool(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetPool(chi.URLParam(r, "pool_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListPools(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListPoolsByCluster(chi.URLParam(r, "cluster_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "pools", items)
}

func (app *Application) UpdatePool(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdatePool(chi.URLParam(r, "pool_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeletePool(w http.ResponseWriter, r *http.Request) {
	poolID := chi.URLParam(r, "pool_id")
	// The Scaleway SDK expects DELETE to return the pool object so it can
	// poll for deletion completion using the pool ID from the response.
	out, err := app.repo.GetPool(poolID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if err := app.repo.DeletePool(poolID); err != nil {
		writeDomainError(w, err)
		return
	}
	out["status"] = "deleting"
	writeJSON(w, http.StatusOK, out)
}
