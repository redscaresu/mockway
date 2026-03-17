variable "app_name" {
  type = string
}

variable "app_description" {
  type    = string
  default = ""
}

resource "scaleway_iam_application" "app" {
  name        = var.app_name
  description = var.app_description
}
