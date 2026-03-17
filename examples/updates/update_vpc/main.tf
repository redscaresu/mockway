variable "vpc_name" {
  type = string
}

resource "scaleway_vpc" "vpc" {
  name = var.vpc_name
}
