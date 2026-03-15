# CROSS-STATE ORPHAN — step 1 of 2: the "platform" workspace.
#
# This workspace owns the shared IAM application used by multiple teams.
# The app workspace (../app) reads this application's ID via terraform_remote_state,
# making the cross-workspace dependency explicit and visible in code.
#
# ── How to reproduce the failure ──────────────────────────────────────────────
#
#   # 1. Apply the platform workspace (this file):
#   cd platform
#   terraform init && terraform apply -auto-approve
#
#   # 2. Apply the app workspace — it reads application_id from ../platform/terraform.tfstate:
#   cd ../app
#   terraform init && terraform apply -auto-approve
#
#   # 3. Now tear down the platform workspace:
#   cd ../platform
#   terraform destroy -auto-approve
#
#   Expected:
#     Error: deleting scaleway_iam_application.shared
#       scaleway-sdk-go: http error 409: conflict: cannot delete: dependents exist
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — the platform config is syntactically valid
#   terraform plan      ✓ passes — shows 1 resource to destroy, looks clean
#
#   Terraform only knows about resources in its own state file. The API key and
#   policy that reference this application live in the app workspace's state —
#   completely invisible to the platform workspace's plan.
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   mockway holds a single in-memory state for all workspaces on the same port.
#   When the platform workspace tries to delete the IAM application, mockway
#   sees that the API key and policy from the app workspace still reference it
#   and returns 409. This matches the real Scaleway API behaviour.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Destroy dependent workspaces before the platform workspace:
#     cd app && terraform destroy -auto-approve
#     cd ../platform && terraform destroy -auto-approve

resource "scaleway_iam_application" "shared" {
  name        = "platform-service"
  description = "Shared IAM application — referenced by multiple workspaces"
}

output "application_id" {
  value       = scaleway_iam_application.shared.id
  description = "Read by the app workspace via terraform_remote_state."
}
