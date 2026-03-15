# BROKEN: RDB database references an instance from a destroyed workspace.
#
# A realistic cross-workspace ordering failure: the RDB instance lives in a
# "platform" Terraform workspace (or module). The database lives in an "app"
# workspace. When the platform workspace is torn down first, the app workspace
# still holds an instance_id that no longer exists. The ID is a valid UUID in
# the correct region-prefixed format, so no static analysis can detect the stale
# reference.
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — instance_id is a valid string
#   terraform plan      ✓ passes — the UUID is syntactically valid; Terraform
#                                  cannot verify it exists without calling the API
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform apply
#   ...
#   Error: creating scaleway_rdb_database.broken
#     scaleway-sdk-go: http error 404: not_found: referenced resource not found
#
#   mockway rejects the database because no RDB instance with that UUID exists
#   in its state. The FK constraint on rdb_databases.instance_id fails.
#
# ── Why this matters ──────────────────────────────────────────────────────────
#
#   The database is never created. Application migrations that rely on this
#   database at deploy time will fail with a connection error — the root cause
#   is buried in Terraform state history, not the current config.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Always apply the platform workspace (RDB instance) before the app workspace
#   (database), and destroy the app workspace before tearing down the platform
#   workspace. Use terraform_remote_state to make the dependency explicit:
#
#     data "terraform_remote_state" "platform" {
#       backend = "local"
#       config  = { path = "../platform/terraform.tfstate" }
#     }
#
#     resource "scaleway_rdb_database" "app" {
#       instance_id = data.terraform_remote_state.platform.outputs.instance_id
#       name        = "appdb"
#     }

resource "scaleway_rdb_database" "broken" {
  name = "appdb"

  # Stale instance ID from a destroyed workspace — valid region-prefixed UUID
  # format (passes terraform validate and plan), but the instance no longer
  # exists in mockway's state.
  instance_id = "fr-par/550e8400-e29b-41d4-a716-446655440000"
}
