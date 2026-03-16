# Load Balancer with Route: LB + two backends + frontend + route rule.
#
# The route directs requests matching a host header to a specific backend.
# Dependency order enforced by resource references:
#   apply:   lb → backend_a → backend_b → frontend → route
#   destroy: route → frontend → backend_a/backend_b → lb

resource "scaleway_lb" "lb" {
  name = "example-lb"
  type = "LB-S"
}

resource "scaleway_lb_backend" "backend_a" {
  lb_id            = scaleway_lb.lb.id
  name             = "backend-a"
  forward_protocol = "http"
  forward_port     = 80
}

resource "scaleway_lb_backend" "backend_b" {
  lb_id            = scaleway_lb.lb.id
  name             = "backend-b"
  forward_protocol = "http"
  forward_port     = 8080
}

resource "scaleway_lb_frontend" "frontend" {
  lb_id        = scaleway_lb.lb.id
  backend_id   = scaleway_lb_backend.backend_a.id
  name         = "example-frontend"
  inbound_port = 80
}

resource "scaleway_lb_route" "route" {
  frontend_id       = scaleway_lb_frontend.frontend.id
  backend_id        = scaleway_lb_backend.backend_b.id
  match_host_header = "api.example.com"
}
