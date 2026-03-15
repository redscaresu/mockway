# CROSS-STATE ORPHAN — step 2 of 2: the "app" workspace.
#
# This workspace reads the shared IAM application ID directly from the platform
# workspace's state file using terraform_remote_state. This makes the
# cross-workspace dependency explicit and visible in code — unlike a variable
# passed on the CLI, you can see exactly which output is consumed and where
# it comes from.
#
# The failure appears when the platform workspace is destroyed while this
# workspace is still applied. See ../platform/main.tf for full steps.

data "terraform_remote_state" "platform" {
  backend = "local"
  config = {
    # Reads the platform workspace's local state file.
    # Both workspaces must share the same mockway instance (same port).
    path = "${path.module}/../platform/terraform.tfstate"
  }
}

resource "scaleway_iam_api_key" "app" {
  # Consumed directly from the platform workspace's output — the dependency
  # chain is: platform state → application_id → this API key.
  application_id = data.terraform_remote_state.platform.outputs.application_id
  description    = "API key for the app service"
}

resource "scaleway_iam_policy" "app" {
  name           = "app-policy"
  application_id = data.terraform_remote_state.platform.outputs.application_id

  rule {
    organization_id      = "00000000-0000-0000-0000-000000000000"
    permission_set_names = ["AllProductsFullAccess"]
  }
}

output "api_key_id" {
  value = scaleway_iam_api_key.app.id
}
