# BROKEN: VPC destroyed before its private network, causing a 409.
#
# This failure is common in multi-workspace or multi-module setups where the
# VPC lives in a "platform" state file and the private networks live in an
# "app" state file. The VPC workspace is torn down first, leaving private
# networks that still reference the deleted VPC.
#
# To reproduce here: apply both resources, remove the private network from
# Terraform's view (simulating it being managed in a separate workspace), then
# attempt to destroy only the VPC:
#
#   terraform apply -auto-approve
#   terraform state rm scaleway_vpc_private_network.pn  # simulate separate workspace
#   terraform destroy -target scaleway_vpc.vpc -auto-approve
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — the config is syntactically correct
#   terraform plan      ✓ passes — after state rm, Terraform sees only the VPC;
#                                  a destroy of 1 resource looks fine
#   terraform destroy   ✓ passes — a full destroy (without state rm) works because
#                                  Terraform sees the vpc_id reference and destroys
#                                  the private network first, then the VPC
#
#   The failure only surfaces when the private network is in a SEPARATE state file
#   (different workspace or module) and the VPC workspace is torn down first.
#   terraform state rm simulates this split-state scenario locally.
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform state rm scaleway_vpc_private_network.pn
#   $ terraform destroy -target scaleway_vpc.vpc -auto-approve
#   ...
#   Error: deleting scaleway_vpc.vpc
#     scaleway-sdk-go: http error 409: conflict: cannot delete: dependents exist
#
#   After state rm, Terraform no longer knows about the private network and sends
#   a bare DELETE for the VPC. mockway refuses because scaleway_vpc_private_network.pn
#   still holds a foreign key reference to it.
#
# ── Why this matters ──────────────────────────────────────────────────────────
#
#   Real Scaleway may return a similar 409, or may delete the VPC and leave the
#   private network in a broken state. mockway's strict FK enforcement catches
#   the wrong destroy order every time, locally, before any prod resources are
#   affected.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Always destroy child resources before their parents:
#     terraform destroy -target scaleway_vpc_private_network.pn -auto-approve
#     terraform destroy -target scaleway_vpc.vpc -auto-approve
#
#   Or in a multi-workspace setup, destroy the app workspace (which owns the
#   private networks) before destroying the platform workspace (which owns the VPC).

resource "scaleway_vpc" "vpc" {
  name = "example-vpc"
}

# This private network holds a FK reference to the VPC.
# Destroying the VPC while this resource still exists → 409.
resource "scaleway_vpc_private_network" "pn" {
  name   = "example-pn"
  vpc_id = scaleway_vpc.vpc.id
}

output "vpc_id" {
  value       = scaleway_vpc.vpc.id
  description = "After apply: terraform state rm scaleway_vpc_private_network.pn && terraform destroy -target scaleway_vpc.vpc"
}
