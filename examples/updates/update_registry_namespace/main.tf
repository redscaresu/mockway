variable "ns_description" {
  type    = string
  default = ""
}

variable "is_public" {
  type    = bool
  default = false
}

resource "scaleway_registry_namespace" "ns" {
  name        = "update-test-ns"
  description = var.ns_description
  is_public   = var.is_public
}
