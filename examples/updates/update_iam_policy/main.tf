variable "policy_name" {
  type = string
}

variable "policy_description" {
  type    = string
  default = ""
}

resource "scaleway_iam_application" "app" {
  name = "policy-update-app"
}

resource "scaleway_iam_policy" "policy" {
  name           = var.policy_name
  description    = var.policy_description
  application_id = scaleway_iam_application.app.id

  rule {
    organization_id = "00000000-0000-0000-0000-000000000000"
    permission_set_names = ["AllProductsFullAccess"]
  }
}
