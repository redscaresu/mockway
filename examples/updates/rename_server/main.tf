# Update scenario: rename a server in-place.

variable "server_name" {
  type = string
}

resource "scaleway_instance_security_group" "sg" {
  name                    = "sg-for-update-test"
  inbound_default_policy  = "drop"
  outbound_default_policy = "accept"
}

resource "scaleway_instance_server" "web" {
  name              = var.server_name
  type              = "DEV1-S"
  image             = "ubuntu_noble"
  security_group_id = scaleway_instance_security_group.sg.id
}
