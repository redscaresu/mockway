# Kubernetes cluster example: cluster + node pool.
#
# Dependency order:
#   apply:   cluster first, then pool
#   destroy: pool removed first (mockway returns 409 if cluster has pools), then cluster
#
# delete_additional_resources is required by the Scaleway provider.

resource "scaleway_k8s_cluster" "cluster" {
  name                        = "example-cluster"
  version                     = "1.31.2"
  cni                         = "cilium"
  delete_additional_resources = false
}

resource "scaleway_k8s_pool" "pool" {
  cluster_id = scaleway_k8s_cluster.cluster.id
  name       = "example-pool"
  node_type  = "DEV1-M"
  size       = 1
}

output "cluster_id" {
  value = scaleway_k8s_cluster.cluster.id
}

output "pool_id" {
  value = scaleway_k8s_pool.pool.id
}
