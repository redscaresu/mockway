# BROKEN: RDB read replica references an instance from a destroyed workspace.
#
# Read replicas are often added to a database long after the primary is
# provisioned — and the primary may live in a separate Terraform workspace.
# If the platform workspace is rebuilt (instance recreated with a new UUID)
# while the app workspace still holds the old instance_id, the read replica
# creation fails silently at apply time.
#
# The UUID is in the correct region-prefixed format, so terraform validate and
# plan both pass. The failure only surfaces when mockway (or real Scaleway)
# checks whether the instance exists.
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — instance_id is a valid region-prefixed UUID
#   terraform plan      ✓ passes — Terraform cannot verify the instance exists
#                                  without calling the Scaleway RDB API
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform apply
#   ...
#   Error: creating scaleway_rdb_read_replica.broken
#     scaleway-sdk-go: http error 404: not_found: referenced resource not found
#
#   mockway rejects the read replica because no RDB instance with that UUID
#   exists in its state. The FK constraint on rdb_read_replicas.instance_id fails.
#
# ── Why this matters ──────────────────────────────────────────────────────────
#
#   Without the read replica, applications pointed at the replica endpoint fail
#   to connect. The primary is unaffected, so the error is not immediately
#   obvious — it only surfaces when read-heavy queries hit the missing replica.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Use terraform_remote_state to read the live instance ID from the platform
#   workspace rather than hardcoding it:
#
#     data "terraform_remote_state" "platform" {
#       backend = "local"
#       config  = { path = "../platform/terraform.tfstate" }
#     }
#
#     resource "scaleway_rdb_read_replica" "replica" {
#       instance_id = data.terraform_remote_state.platform.outputs.instance_id
#       direct_access {}
#     }

resource "scaleway_rdb_read_replica" "broken" {
  # Stale instance ID from a rebuilt/destroyed platform workspace — valid
  # region-prefixed UUID format (passes terraform validate and plan), but the
  # instance no longer exists in mockway's state.
  instance_id = "fr-par/550e8400-e29b-41d4-a716-446655440000"

  direct_access {}
}
