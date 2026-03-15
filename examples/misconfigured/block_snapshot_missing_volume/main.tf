# BROKEN: block snapshot references a volume UUID that doesn't exist.
#
# The volume_id is a plain string — a UUID copy-pasted from the Scaleway
# console or left over from a deleted resource. No static analysis can
# distinguish a valid UUID from a stale one before apply.
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — volume_id is a valid string attribute
#   terraform plan      ✓ passes — Terraform resolves the reference to a UUID
#                                  but cannot verify the volume exists in Scaleway
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform apply
#   ...
#   Error: creating scaleway_block_snapshot.broken
#     scaleway-sdk-go: http error 404: not_found: referenced resource not found
#
#   mockway rejects the snapshot because "aabbccdd-..." has never been created
#   as a block volume. The FK constraint on block_snapshots.volume_id fails.
#
# ── Why this matters ──────────────────────────────────────────────────────────
#
#   On real Scaleway the snapshot API call returns 404 at apply time. The failure
#   is indistinguishable from a network error in CI, and you may spend time
#   investigating the wrong thing. mockway surfaces it in local testing before
#   any remote call is made.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Create the volume as a Terraform resource and reference it:
#
#     resource "scaleway_block_volume" "vol" {
#       name = "my-volume"
#       size = 20
#     }
#
#     resource "scaleway_block_snapshot" "snap" {
#       name      = "my-snapshot"
#       volume_id = scaleway_block_volume.vol.id  # <-- reference
#     }

resource "scaleway_block_snapshot" "broken" {
  name = "broken-snapshot"

  # Wrong: hard-coded UUID that does not exist in mockway's state.
  volume_id = "aabbccdd-1234-1234-1234-aabbccddeeff"
}
