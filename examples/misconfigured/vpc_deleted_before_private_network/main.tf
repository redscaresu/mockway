# BROKEN: VPC destroyed before its private network, causing a 409.
#
# This failure is common in multi-workspace or multi-module setups where the
# VPC lives in a "platform" state file and the private networks live in an
# "app" state file. The VPC workspace is torn down first, leaving private
# networks that still reference the deleted VPC.
#
# To reproduce here: apply both resources normally, then destroy only the VPC:
#
#   terraform apply -auto-approve
#   terraform destroy -target scaleway_vpc.vpc -auto-approve
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — the config is syntactically correct
#   terraform plan      ✓ passes — a targeted destroy of the VPC is valid to plan
#   terraform destroy   ✓ passes — a full destroy works because Terraform sees the
#                                  vpc_id reference and destroys the private network
#                                  first, then the VPC
#
#   The failure only surfaces when the VPC is destroyed out of order: a
#   -target destroy, a manual console deletion, or a separate workspace teardown.
#   None of these scenarios are visible to terraform validate or terraform plan.
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform destroy -target scaleway_vpc.vpc -auto-approve
#   ...
#   Error: deleting scaleway_vpc.vpc
#     scaleway-sdk-go: http error 409: conflict: cannot delete: dependents exist
#
#   mockway refuses to delete the VPC because scaleway_vpc_private_network.pn
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
  description = "Try: terraform destroy -target scaleway_vpc.vpc to see the 409."
}
