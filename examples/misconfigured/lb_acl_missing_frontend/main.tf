# BROKEN: LB ACL's frontend_id points at the LB instead of the frontend.
#
# Both scaleway_lb.lb.id and scaleway_lb_frontend.fe.id are UUIDs. When writing
# the ACL resource, it is easy to autocomplete to the wrong one — especially
# since the LB and frontend often share a naming prefix. The ACL is logically
# attached to a frontend, but the attribute is just a string ID.
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — frontend_id is a valid string reference
#   terraform plan      ✓ passes — Terraform resolves both UUIDs correctly;
#                                  it cannot verify which one is a frontend ID
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform apply
#   ...
#   scaleway_lb.lb: Creation complete
#   scaleway_lb_backend.backend: Creation complete
#   scaleway_lb_frontend.fe: Creation complete
#   Error: creating scaleway_lb_acl.broken
#     scaleway-sdk-go: http error 404: not_found: referenced resource not found
#
#   mockway rejects the ACL because the value passed as frontend_id is the LB's
#   UUID, and no frontend exists with that ID.
#
# ── Why this matters ──────────────────────────────────────────────────────────
#
#   An LB ACL that fails to attach leaves the frontend with no access controls.
#   Traffic that was meant to be blocked (e.g. by IP allowlist) reaches the
#   backend unfiltered. The apply error is clear, but without a pre-apply check
#   this security gap can persist unnoticed in a partially-applied state.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   In scaleway_lb_acl.broken, change:
#     frontend_id = scaleway_lb.lb.id        # wrong: this is the LB's ID
#   To:
#     frontend_id = scaleway_lb_frontend.fe.id

resource "scaleway_lb_ip" "ip" {}

resource "scaleway_lb" "lb" {
  ip_id = scaleway_lb_ip.ip.id
  name  = "example-lb"
  type  = "LB-S"
}

resource "scaleway_lb_backend" "backend" {
  lb_id            = scaleway_lb.lb.id
  name             = "example-backend"
  forward_protocol = "http"
  forward_port     = 80
}

resource "scaleway_lb_frontend" "fe" {
  lb_id        = scaleway_lb.lb.id
  backend_id   = scaleway_lb_backend.backend.id
  name         = "example-frontend"
  inbound_port = 80
}

resource "scaleway_lb_acl" "broken" {
  # Wrong: points at the LB's UUID instead of the frontend's UUID.
  # Both are valid Terraform references — easy to mistype.
  # frontend_id = scaleway_lb.lb.id

  # The correct reference:
  frontend_id = scaleway_lb_frontend.fe.id

  name  = "deny-all-acl"
  index = 1

  action {
    type = "deny"
  }

  match {
    ip_subnet = ["0.0.0.0/0"]
  }
}
