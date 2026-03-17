resource "scaleway_vpc" "vpc" {
  name = "mockway-vpc"
}

resource "scaleway_vpc_private_network" "pn" {
  name   = "mockway-pn"
  vpc_id = scaleway_vpc.vpc.id
}

resource "scaleway_lb" "lb" {
  name = "mockway-lb"
  type = "LB-S"
}

resource "scaleway_lb_private_network" "attach" {
  lb_id              = scaleway_lb.lb.id
  private_network_id = scaleway_vpc_private_network.pn.id
}
