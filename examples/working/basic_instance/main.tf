# Happy path: a server with a properly referenced security group.
#
# Both resources are in the same module. Terraform's implicit dependency on
# scaleway_instance_security_group.sg.id means:
#
#   apply:   security group is created first, then the server
#   destroy: server is removed first, then the security group
#
# mockway enforces this at the HTTP level:
#   - Creating the server with a non-existent security group ID → 404
#   - Deleting the security group while the server still exists → 409
#
# Using resource references instead of hard-coded UUIDs keeps the Terraform
# dependency graph in sync with mockway's FK constraints.

resource "scaleway_instance_security_group" "sg" {
  name                    = "example-sg"
  inbound_default_policy  = "drop"
  outbound_default_policy = "accept"
  stateful                = true

  inbound_rule {
    action   = "accept"
    port     = 22
    protocol = "TCP"
  }
}

resource "scaleway_instance_server" "web" {
  name  = "example-server"
  type  = "DEV1-S"
  image = "ubuntu_noble"

  security_group_id = scaleway_instance_security_group.sg.id
}

output "server_id" {
  value = scaleway_instance_server.web.id
}
