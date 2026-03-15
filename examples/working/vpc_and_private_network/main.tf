# Happy path: VPC → private network → server with private NIC attached.
#
# The full FK dependency chain, encoded via resource references:
#
#   scaleway_vpc_private_network.pn    depends on  scaleway_vpc.vpc
#   scaleway_instance_server.web       depends on  scaleway_instance_security_group.sg
#   scaleway_instance_private_nic.nic  depends on  scaleway_instance_server.web
#                                      AND         scaleway_vpc_private_network.pn
#
# mockway enforces both FK parents on the private NIC:
#   - server_id must exist in instance_servers
#   - private_network_id must exist in private_networks (from the VPC API)
#
# Destroy order (automatic via resource references):
#   private NIC → server → security group
#                → private network → VPC

resource "scaleway_vpc" "vpc" {
  name = "example-vpc"
}

resource "scaleway_vpc_private_network" "pn" {
  name   = "example-pn"
  vpc_id = scaleway_vpc.vpc.id
}

resource "scaleway_instance_security_group" "sg" {
  name                    = "example-sg"
  inbound_default_policy  = "drop"
  outbound_default_policy = "accept"
  stateful                = true
}

resource "scaleway_instance_server" "web" {
  name  = "example-server"
  type  = "DEV1-S"
  image = "ubuntu_noble"

  security_group_id = scaleway_instance_security_group.sg.id
}

resource "scaleway_instance_private_nic" "nic" {
  server_id          = scaleway_instance_server.web.id
  private_network_id = scaleway_vpc_private_network.pn.id
}

output "server_id" {
  value = scaleway_instance_server.web.id
}

output "private_network_id" {
  value = scaleway_vpc_private_network.pn.id
}
