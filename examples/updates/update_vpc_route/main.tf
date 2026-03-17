variable "route_description" {
  type = string
}

resource "scaleway_vpc" "vpc" {
  name = "route-update-vpc"
}

resource "scaleway_vpc_private_network" "pn" {
  name   = "route-update-pn"
  vpc_id = scaleway_vpc.vpc.id
}

resource "scaleway_vpc_route" "route" {
  vpc_id                     = scaleway_vpc.vpc.id
  destination                = "10.0.0.0/8"
  description                = var.route_description
  nexthop_private_network_id = scaleway_vpc_private_network.pn.id
}
