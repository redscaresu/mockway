# A server whose ip_id moves from one flexible IP to another in place.
# The provider detaches and attaches through PATCH /ips/{id} {server},
# never through the server, so the server's public_ips must follow the
# IPs or the converge plan proposes the ip_id change again.

variable "ip" {
  type = string
}

resource "scaleway_instance_ip" "a" {}

resource "scaleway_instance_ip" "b" {}

locals {
  ips = {
    a = scaleway_instance_ip.a.id
    b = scaleway_instance_ip.b.id
  }
}

resource "scaleway_instance_server" "web" {
  name  = "web"
  type  = "DEV1-S"
  image = "ubuntu_noble"
  ip_id = local.ips[var.ip]
}
