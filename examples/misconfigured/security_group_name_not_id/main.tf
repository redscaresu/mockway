# BROKEN: server's security_group_id set to the security group's name, not its ID.
#
# This is an easy autocomplete mistake: tab-completion or IDE suggestions show
# both .id and .name for a resource. A platform engineer picks .name — it is a
# valid string attribute and the config looks correct — but Scaleway expects a
# UUID, not a human-readable name.
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — security_group_id is typed as string; a name
#                                  is a valid string
#   terraform plan      ✓ passes — Terraform resolves the reference; it cannot
#                                  know the resolved value isn't a UUID
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform apply
#   ...
#   scaleway_instance_security_group.sg: Creation complete
#   Error: creating scaleway_instance_server.broken
#     scaleway-sdk-go: http error 404: not_found: referenced resource not found
#
#   mockway stores security groups by UUID. The value "example-sg" (the name)
#   never matches any security_group_id in the database — FK check fails.
#
# ── Why this matters ──────────────────────────────────────────────────────────
#
#   On real Scaleway the server may be created with the default (less restrictive)
#   security group instead, silently bypassing the hardened policy. The error
#   is invisible at plan time and may not be caught in review.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Change:
#     security_group_id = scaleway_instance_security_group.sg.name   # wrong
#   To:
#     security_group_id = scaleway_instance_security_group.sg.id     # correct

resource "scaleway_instance_security_group" "sg" {
  name                   = "example-sg"
  inbound_default_policy = "drop"
  stateful               = true
}

resource "scaleway_instance_server" "broken" {
  name  = "example-server"
  type  = "DEV1-S"
  image = "ubuntu_noble"

  # Wrong: .name resolves to "example-sg" — a string, not a UUID.
  # Scaleway expects the UUID returned by .id.
  security_group_id = scaleway_instance_security_group.sg.name
}
