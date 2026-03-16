# Load Balancer with ACL: LB + backend + frontend + ACL rule.
#
# Dependency order enforced by resource references:
#   apply:   lb → backend → frontend → acl
#   destroy: acl → frontend → backend → lb

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
  lb_id          = scaleway_lb.lb.id
  backend_id     = scaleway_lb_backend.backend.id
  name           = "example-frontend"
  inbound_port   = 80
  external_acls  = true
}

resource "scaleway_lb_acl" "allow_internal" {
  frontend_id = scaleway_lb_frontend.frontend.id
  name        = "allow-internal"
  index       = 1

  action {
    type = "allow"
  }

  match {
    ip_subnet = ["10.0.0.0/8"]
  }
}

resource "scaleway_lb_acl" "deny_all" {
  frontend_id = scaleway_lb_frontend.frontend.id
  name        = "deny-all"
  index       = 2

  action {
    type = "deny"
  }

  match {
    ip_subnet = ["0.0.0.0/0"]
  }
}
