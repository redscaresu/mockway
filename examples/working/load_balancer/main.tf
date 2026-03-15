# Load Balancer example: LB + backend + frontend.
#
# Dependency order enforced by resource references:
#   apply:   lb → backend → frontend
#   destroy: frontend → backend → lb
#
# The frontend references the backend_id, so Terraform creates them in order.

resource "scaleway_lb" "lb" {
  name = "example-lb"
  type = "LB-S"
}

resource "scaleway_lb_backend" "backend" {
  lb_id            = scaleway_lb.lb.id
  name             = "example-backend"
  forward_protocol = "http"
  forward_port     = 80
}

resource "scaleway_lb_frontend" "frontend" {
  lb_id        = scaleway_lb.lb.id
  backend_id   = scaleway_lb_backend.backend.id
  name         = "example-frontend"
  inbound_port = 80
}

output "lb_id" {
  value = scaleway_lb.lb.id
}
