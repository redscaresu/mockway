# BROKEN: hard-coded security group UUID.
#
# The security_group_id is copied from the Scaleway console (or another
# environment's state) rather than referencing a resource in this config.
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — the attribute type is correct (a string)
#   terraform plan      ✓ passes — Terraform cannot know the UUID is invalid
#                                  without calling the Scaleway API
#
#   The broken reference is invisible until apply time.
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform apply
#   ...
#   Error: creating scaleway_instance_server.broken
#     scaleway-sdk-go: http error 404: not_found: referenced resource not found
#
#   mockway rejects the server creation because "aabbccdd-..." has never been
#   created. The FK constraint on instance_servers.security_group_id fails.
#
# ── Why this matters ──────────────────────────────────────────────────────────
#
#   On a real Scaleway account this might silently attach the default security
#   group, leaving the server less hardened than intended, with no error raised.
#   mockway surfaces the broken reference immediately so it never reaches prod.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Define the security group as a resource in this config and reference it:
#
#     resource "scaleway_instance_security_group" "sg" {
#       name                   = "example-sg"
#       inbound_default_policy = "drop"
#       stateful               = true
#     }
#
#     resource "scaleway_instance_server" "web" {
#       ...
#       security_group_id = scaleway_instance_security_group.sg.id  # <-- reference
#     }

resource "scaleway_instance_server" "broken" {
  name  = "broken-server"
  type  = "DEV1-S"
  image = "ubuntu_noble"

  # Wrong: literal UUID that does not exist in mockway's state.
  security_group_id = "aabbccdd-1234-1234-1234-aabbccddeeff"
}
