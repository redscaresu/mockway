# IAM full example: application + api-key + policy (with rules) + ssh-key.
#
# Dependency order:
#   apply:   application first, then api-key and policy (both reference it)
#   destroy: api-key and policy removed first, then application
#
# mockway enforces FK constraints:
#   - Creating an api-key with a non-existent application_id → 404
#   - Deleting an application while api-keys or policies still reference it → 409
#
# Rules sent in the scaleway_iam_policy body are stored by mockway and returned
# via GET /iam/v1alpha1/rules?policy_id=<id> on the next plan refresh, which
# prevents drift on subsequent terraform plan runs.

resource "scaleway_iam_application" "app" {
  name        = "example-app"
  description = "Example IAM application"
}

resource "scaleway_iam_api_key" "key" {
  application_id = scaleway_iam_application.app.id
  description    = "API key for example-app"
}

resource "scaleway_iam_policy" "policy" {
  name           = "example-policy"
  description    = "Full access policy for example-app"
  application_id = scaleway_iam_application.app.id

  rule {
    organization_id      = "00000000-0000-0000-0000-000000000000"
    permission_set_names = ["AllProductsFullAccess"]
  }
}

resource "scaleway_iam_ssh_key" "ssh" {
  name       = "example-ssh-key"
  public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GkZL user@example"
}

output "application_id" {
  value = scaleway_iam_application.app.id
}

output "api_key_access_key" {
  value = scaleway_iam_api_key.key.id
}
