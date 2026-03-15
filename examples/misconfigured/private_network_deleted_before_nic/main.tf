# BROKEN: private network destroyed while a server NIC is still attached to it.
#
# This occurs in layered infrastructure: the private network is managed in one
# Terraform workspace (or module) while the server NIC is managed in another.
# When the networking layer is torn down first, the compute layer is left with
# NICs pointing at a deleted network — causing silent connectivity loss.
#
# To reproduce: apply all resources, remove the NIC from Terraform's view
# (simulating it being managed in a separate workspace), then destroy only the
# private network:
#
#   terraform apply -auto-approve
#   terraform state rm scaleway_instance_private_nic.nic  # simulate separate workspace
#   terraform destroy -target scaleway_vpc_private_network.pn -auto-approve
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — the config is syntactically correct
#   terraform plan      ✓ passes — after state rm, Terraform sees only the PN;
#                                  a destroy of 1 resource looks fine
#   terraform destroy   ✓ passes — a full destroy (without state rm) works because
#                                  Terraform computes the correct dependency order
#                                  and removes the NIC before the private network
#
#   The failure only surfaces when the NIC is in a SEPARATE state file (different
#   workspace) and the networking workspace is torn down first. terraform state rm
#   simulates this split-state scenario locally.
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform state rm scaleway_instance_private_nic.nic
#   $ terraform destroy -target scaleway_vpc_private_network.pn -auto-approve
#   ...
#   Error: deleting scaleway_vpc_private_network.pn
#     scaleway-sdk-go: http error 409: conflict: cannot delete: dependents exist
#
#   After state rm, Terraform no longer knows about the NIC and sends a bare
#   DELETE for the private network. mockway refuses because instance_private_nics
#   still holds a FK reference to it. The NIC must be detached first.
#
# ── Why this matters ──────────────────────────────────────────────────────────
#
#   On real Scaleway the private network deletion may succeed while the NIC
#   still exists — leaving the server with a dangling NIC and no L2 connectivity.
#   The failure is silent at destroy time and only discovered when the application
#   tries to reach the database or service on the private network.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Always destroy server NICs before their private network:
#     terraform destroy -target scaleway_instance_private_nic.nic -auto-approve
#     terraform destroy -target scaleway_vpc_private_network.pn -auto-approve
#
#   In a multi-workspace setup, destroy the compute workspace (NICs) before
#   destroying the networking workspace (private networks).

resource "scaleway_vpc" "vpc" {
  name = "example-vpc"
}

resource "scaleway_vpc_private_network" "pn" {
  name   = "example-pn"
  vpc_id = scaleway_vpc.vpc.id
}

resource "scaleway_instance_security_group" "sg" {
  name                   = "example-sg"
  inbound_default_policy = "drop"
}

resource "scaleway_instance_server" "server" {
  name              = "example-server"
  type              = "DEV1-S"
  image             = "ubuntu_noble"
  security_group_id = scaleway_instance_security_group.sg.id
}

# The NIC holds a FK reference to the private network.
# Destroying the private network while this NIC exists → 409.
resource "scaleway_instance_private_nic" "nic" {
  server_id          = scaleway_instance_server.server.id
  private_network_id = scaleway_vpc_private_network.pn.id
}

output "private_network_id" {
  value       = scaleway_vpc_private_network.pn.id
  description = "Try: terraform destroy -target scaleway_vpc_private_network.pn to see the 409."
}
