# VPC with a custom route.

resource "scaleway_vpc" "vpc" {
  name = "route-test-vpc"
}

resource "scaleway_vpc_private_network" "pn" {
  name   = "route-test-pn"
  vpc_id = scaleway_vpc.vpc.id
}

resource "scaleway_vpc_route" "route" {
  vpc_id                     = scaleway_vpc.vpc.id
  destination                = "10.0.0.0/8"
  description                = "test custom route"
  nexthop_private_network_id = scaleway_vpc_private_network.pn.id
}

output "route_id" {
  value = scaleway_vpc_route.route.id
}
