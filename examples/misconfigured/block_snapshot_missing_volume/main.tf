# BROKEN: block snapshot references a hard-coded volume UUID that doesn't exist.
#
# A snapshot is taken of a volume that was either created in another workspace,
# deleted, or simply copy-pasted from Scaleway console output. Because both
# scaleway_block_volume.id and the hard-coded UUID are plain strings, no
# tooling can distinguish them before apply.
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
