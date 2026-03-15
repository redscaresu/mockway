# BROKEN: IAM policy linked to an application from a different environment.
#
# A common migration pattern: the application_id is taken from the Scaleway
# console of the staging environment and placed into the production config.
# The UUID is syntactically valid, so terraform validate and plan both pass —
# but the application does not exist in the target account.
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — application_id is a valid UUID string
#   terraform plan      ✓ passes — Terraform cannot verify the UUID belongs to
#                                  an existing application without calling the API
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform apply
#   ...
#   Error: creating scaleway_iam_policy.broken
#     scaleway-sdk-go: http error 404: not_found: referenced resource not found
#
#   mockway rejects the policy because no IAM application with that UUID was
#   ever created. The FK constraint on iam_policies.application_id fails.
#
# ── Why this matters ──────────────────────────────────────────────────────────
#
#   The policy is never created, so the application has no permissions. CI/CD
#   pipelines authenticating as this application will receive access-denied
#   errors that are hard to trace back to a missing policy.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Create the application as a resource in the same config and reference it:
#
#     resource "scaleway_iam_application" "svc" {
#       name = "my-service"
#     }
#
#     resource "scaleway_iam_policy" "policy" {
#       name           = "my-policy"
#       application_id = scaleway_iam_application.svc.id  # <-- reference
#       rule { ... }
#     }

resource "scaleway_iam_policy" "broken" {
  name = "my-service-policy"

  # Application ID copied from the staging Scaleway console — valid UUID format
  # (passes terraform validate and plan), but this application does not exist
  # in the current account/mockway state.
  application_id = "550e8400-e29b-41d4-a716-446655440000"

  rule {
    project_ids          = ["00000000-0000-0000-0000-000000000000"]
    permission_set_names = ["ObjectStorageFullAccess"]
  }
}
