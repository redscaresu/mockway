variable "pn_name" {
  type = string
}

resource "scaleway_vpc" "vpc" {
  name = "pn-update-vpc"
}

resource "scaleway_vpc_private_network" "pn" {
  name   = var.pn_name
  vpc_id = scaleway_vpc.vpc.id
}
