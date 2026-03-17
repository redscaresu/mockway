# Update scenario: rename K8s cluster and change tags.

variable "cluster_name" {
  type = string
}

variable "cluster_tags" {
  type = list(string)
}

resource "scaleway_k8s_cluster" "cluster" {
  name                        = var.cluster_name
  version                     = "1.28.15"
  cni                         = "cilium"
  tags                        = var.cluster_tags
  delete_additional_resources = true
}

resource "scaleway_k8s_pool" "pool" {
  cluster_id = scaleway_k8s_cluster.cluster.id
  name       = "default"
  node_type  = "DEV1-M"
  size       = 1
}
