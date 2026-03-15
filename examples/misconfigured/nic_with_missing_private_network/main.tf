# BROKEN: private NIC references a private network that does not exist.
#
# The private_network_id is passed in as a variable. A wrong value — typo,
# stale ID, wrong region — means the server is created but has no private
# network attachment.
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — var.private_network_id is a valid string
#   terraform plan      ✓ passes — the server and NIC are planned successfully;
#                                  Terraform has no way to verify the UUID exists
#                                  in the Scaleway API without calling it
#
#   The broken reference is invisible until apply time.
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform apply
#   ...
#   scaleway_instance_security_group.sg: Creating... [OK]
#   scaleway_instance_server.web: Creating...        [OK]
#   scaleway_instance_private_nic.nic: Creating...
#
#   Error: creating scaleway_instance_private_nic.nic
#     scaleway-sdk-go: http error 404: not_found: referenced resource not found
#
#   The server is created successfully. The NIC fails because mockway enforces
#   TWO FK constraints:
#     - server_id          must exist in instance_servers     ✓ (just created)
#     - private_network_id must exist in private_networks     ✗ (never created)
#
# ── Why this matters ──────────────────────────────────────────────────────────
#
#   Without mockway the server would be created in production with no private
#   network attachment. The misconfiguration is silent and requires manual
#   inspection of the running instance to discover.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Create the private network as a resource and reference it directly:
#
#     resource "scaleway_vpc_private_network" "pn" {
#       name = "example-pn"
#     }
#
#     resource "scaleway_instance_private_nic" "nic" {
#       server_id          = scaleway_instance_server.web.id
#       private_network_id = scaleway_vpc_private_network.pn.id
#     }

variable "private_network_id" {
  type        = string
  description = "ID of the private network to attach to the server."
  default     = "aabbccdd-1234-1234-1234-aabbccddeeff" # Wrong: does not exist in mockway.
}

resource "scaleway_instance_security_group" "sg" {
  name                    = "example-sg"
  inbound_default_policy  = "drop"
  outbound_default_policy = "accept"
  stateful                = true
}

resource "scaleway_instance_server" "web" {
  name  = "example-server"
  type  = "DEV1-S"
  image = "ubuntu_noble"

  security_group_id = scaleway_instance_security_group.sg.id
}

resource "scaleway_instance_private_nic" "nic" {
  server_id = scaleway_instance_server.web.id

  # Wrong: this UUID does not exist in mockway — the private network
  # was never created.
  private_network_id = var.private_network_id
}
