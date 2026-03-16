# Kubernetes cluster with auto-upgrade enabled.
#
# auto_upgrade.enable triggers a request key "enable" in the SDK but the
# response struct reads "enabled" — mockway normalises this so the second plan
# is a no-op (no drift).
#
# A minor version (x.y) is required when auto-upgrade is enabled.

resource "scaleway_k8s_cluster" "cluster" {
  name                        = "example-cluster"
  version                     = "1.31"
  cni                         = "cilium"
  delete_additional_resources = false

  auto_upgrade {
    enable                        = true
    maintenance_window_start_hour = 4
    maintenance_window_day        = "monday"
  }
}

resource "scaleway_k8s_pool" "pool" {
  cluster_id = scaleway_k8s_cluster.cluster.id
  name       = "example-pool"
  node_type  = "DEV1-M"
  size       = 1
}
