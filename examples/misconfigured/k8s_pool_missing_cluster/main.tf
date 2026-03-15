# BROKEN: node pool's cluster_id set to the cluster's name, not its ID.
#
# A platform engineer wires a pool to a cluster using .name instead of .id —
# a common tab-completion mistake. Both attributes are strings, so the config
# is syntactically valid and passes all static checks.
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — cluster_id is typed as string; a name is
#                                  a valid string
#   terraform plan      ✓ passes — Terraform resolves the attribute; it cannot
#                                  verify the resolved value is a UUID
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform apply
#   ...
#   scaleway_k8s_cluster.cluster: Creation complete
#   Error: creating scaleway_k8s_pool.broken
#     scaleway-sdk-go: http error 404: not_found: referenced resource not found
#
#   mockway rejects the pool because "my-cluster" (the name) is not a UUID and
#   does not match any cluster ID in the database.
#
# ── Why this matters ──────────────────────────────────────────────────────────
#
#   The cluster is created but has no worker nodes. Workloads cannot be
#   scheduled and the cluster appears healthy to tooling that only checks the
#   control plane. The error is silent at plan time.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Change:
#     cluster_id = scaleway_k8s_cluster.cluster.name   # wrong
#   To:
#     cluster_id = scaleway_k8s_cluster.cluster.id     # correct

resource "scaleway_k8s_cluster" "cluster" {
  name                        = "my-cluster"
  version                     = "1.31.2"
  cni                         = "cilium"
  delete_additional_resources = false
}

resource "scaleway_k8s_pool" "broken" {
  name      = "my-pool"
  node_type = "DEV1-M"
  size      = 1

  # Wrong: .name resolves to "my-cluster" — a human-readable name, not a UUID.
  # Scaleway expects the UUID returned by .id.
  cluster_id = scaleway_k8s_cluster.cluster.name
}
