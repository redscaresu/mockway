# BROKEN: frontend's backend_id points at the LB instead of the backend.
#
# Both scaleway_lb.lb.id and scaleway_lb_backend.backend.id are UUIDs — a
# natural autocomplete mistake when both resources are in scope. The backend
# is correctly defined; only the cross-reference is wrong.
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — both attributes are valid string references
#   terraform plan      ✓ passes — Terraform resolves both to UUIDs and sees no
#                                  type mismatch; it cannot verify which UUID
#                                  should go where without calling the API
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform apply
#   ...
#   scaleway_lb.lb: Creation complete
#   scaleway_lb_backend.backend: Creation complete
#   Error: creating scaleway_lb_frontend.broken
#     scaleway-sdk-go: http error 404: not_found: referenced resource not found
#
#   mockway rejects the frontend because the value passed as backend_id is the
#   LB's UUID, and no backend exists with that ID.
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   In scaleway_lb_frontend.broken, change:
#     backend_id = scaleway_lb.lb.id          # wrong resource
#   To:
#     backend_id = scaleway_lb_backend.backend.id

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

resource "scaleway_lb_frontend" "broken" {
  lb_id        = scaleway_lb.lb.id
  name         = "example-frontend"
  inbound_port = 80

  # Wrong: points at the LB's UUID instead of the backend's UUID.
  # Both are valid Terraform references that resolve to UUIDs — easy to
  # mistype when autocomplete offers scaleway_lb.lb.id and
  # scaleway_lb_backend.backend.id side by side.
  backend_id = scaleway_lb.lb.id

  # The correct reference:
  # backend_id = scaleway_lb_backend.backend.id
}
