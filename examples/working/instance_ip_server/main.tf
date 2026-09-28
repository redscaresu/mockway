# A server bound to a flexible IP through ip_id.
#
# The data source reads the IP back once the server exists, through the
# same code the resource refreshes with, and its postcondition fails the
# apply -- and every re-plan, which re-reads it -- unless the IP names
# the server it is bound to. The no-op plan alone cannot catch this: the
# IP's server_id is computed, so reading "" on a bound IP is not a diff.

resource "scaleway_instance_ip" "web" {}

resource "scaleway_instance_server" "web" {
  name  = "web"
  type  = "DEV1-S"
  image = "ubuntu_noble"
  ip_id = scaleway_instance_ip.web.id
}

data "scaleway_instance_ip" "web" {
  id         = scaleway_instance_ip.web.id
  depends_on = [scaleway_instance_server.web]

  lifecycle {
    postcondition {
      condition     = self.server_id == scaleway_instance_server.web.id
      error_message = "The IP does not name the server it is bound to: server_id is \"${self.server_id}\"."
    }
  }
}
