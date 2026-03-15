# BROKEN: IAM group membership uses a data source lookup for a user that has
# never been invited to the organisation.
#
# A common real-world pattern: a data source looks up a user by email so the
# config remains portable across environments. The lookup silently fails when
# the user does not exist in the target account (e.g. after an org migration,
# offboarding, or against a fresh environment like mockway).
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — the config is syntactically correct
#   terraform plan      ✗ may fail depending on provider version — but some
#                         provider versions defer the data source read to apply,
#                         meaning the error only surfaces then
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform apply
#   ...
#   Error: reading scaleway_iam_user (email: alice@example.com)
#     scaleway-sdk-go: http error 404: not_found: user not found
#
#   mockway has no users in its initial state. The data source lookup returns
#   404 immediately, before any group or membership is created.
#
# ── Why this matters ──────────────────────────────────────────────────────────
#
#   The group is either not created at all, or created without the expected
#   member. Production services relying on group membership for access control
#   find the member absent — causing access-denied errors or, if a fallback
#   policy is more permissive, unintended privilege escalation.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Ensure the user has been invited to the organisation before running apply.
#   In a managed setup, create the user as a resource first:
#
#     resource "scaleway_iam_user" "alice" {
#       email = "alice@example.com"
#     }
#
#     resource "scaleway_iam_group_membership" "member" {
#       group_id = scaleway_iam_group.team.id
#       user_id  = scaleway_iam_user.alice.id
#     }

# Looks up a user by email — fails if alice has not been invited to this org.
data "scaleway_iam_user" "alice" {
  email = "alice@example.com"
}

resource "scaleway_iam_group" "team" {
  name        = "platform-team"
  description = "Platform engineering group"
}

resource "scaleway_iam_group_membership" "broken" {
  group_id = scaleway_iam_group.team.id

  # Fails when the data source returns no user (404 from mockway).
  user_id = data.scaleway_iam_user.alice.id
}
