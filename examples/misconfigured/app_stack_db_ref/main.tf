# BROKEN: A realistic multi-tier application stack where everything looks correctly
# wired up — except the LB frontend's backend_id points at the LB instead of
# the backend. Both scaleway_lb.main.id and scaleway_lb_backend.app.id are UUIDs;
# autocomplete or a copy-paste easily produces the wrong one.
#
# Resources defined (12 total):
#   IAM:      application, api-key, policy, ssh-key
#   Instance: security-group, server
#   LB:       lb, backend, frontend (BROKEN)
#   RDB:      instance, database, user
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes — both attributes resolve to valid string refs
#   terraform plan      ✓ passes — Terraform sees two UUIDs and has no way to
#                                  know which one should go in backend_id
#
# ── What mockway catches ──────────────────────────────────────────────────────
#
#   $ terraform apply
#   ...
#   scaleway_iam_application.svc: Creation complete
#   scaleway_iam_api_key.svc: Creation complete
#   scaleway_iam_policy.svc: Creation complete
#   scaleway_iam_ssh_key.deploy: Creation complete
#   scaleway_instance_security_group.sg: Creation complete
#   scaleway_instance_server.app: Creation complete
#   scaleway_lb.main: Creation complete
#   scaleway_lb_backend.app: Creation complete
#   scaleway_rdb_instance.db: Creation complete
#   scaleway_rdb_database.app: Creation complete
#   scaleway_rdb_user.app: Creation complete
#   Error: creating scaleway_lb_frontend.app
#     scaleway-sdk-go: http error 404: not_found: referenced resource not found
#
#   11 resources applied before the failure.
#   The partial state must now be cleaned up with terraform destroy.
#
# ── Why it is subtle ──────────────────────────────────────────────────────────
#
#   1. 12 resources across 4 services — hard to audit by eye
#   2. Every other resource reference in the file is correct
#   3. scaleway_lb_frontend.app.lb_id correctly uses scaleway_lb.main.id —
#      the same resource that appears (wrongly) in backend_id one line below
#   4. Both resolve to UUIDs in the plan output; no linter or type check catches it
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   In scaleway_lb_frontend.app, change:
#     backend_id = scaleway_lb.main.id          # wrong resource
#   To:
#     backend_id = scaleway_lb_backend.app.id

# ── IAM ───────────────────────────────────────────────────────────────────────

resource "scaleway_iam_application" "svc" {
  name        = "app-service"
  description = "Service account for the application"
}

resource "scaleway_iam_api_key" "svc" {
  application_id = scaleway_iam_application.svc.id
  description    = "API key for app-service"
}

resource "scaleway_iam_policy" "svc" {
  name           = "app-policy"
  application_id = scaleway_iam_application.svc.id

  rule {
    organization_id      = "00000000-0000-0000-0000-000000000000"
    permission_set_names = ["AllProductsFullAccess"]
  }
}

resource "scaleway_iam_ssh_key" "deploy" {
  name       = "deploy-key"
  public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GkZL deploy@ci"
}

# ── Instance ──────────────────────────────────────────────────────────────────

resource "scaleway_instance_security_group" "sg" {
  name                    = "app-sg"
  inbound_default_policy  = "drop"
  outbound_default_policy = "accept"
  stateful                = true

  inbound_rule {
    action   = "accept"
    port     = 80
    protocol = "TCP"
  }

  inbound_rule {
    action   = "accept"
    port     = 443
    protocol = "TCP"
  }
}

resource "scaleway_instance_server" "app" {
  name              = "app-server"
  type              = "DEV1-S"
  image             = "ubuntu_noble"
  security_group_id = scaleway_instance_security_group.sg.id
}

# ── Load Balancer ─────────────────────────────────────────────────────────────

resource "scaleway_lb" "main" {
  name = "app-lb"
  type = "LB-S"
}

resource "scaleway_lb_backend" "app" {
  lb_id            = scaleway_lb.main.id
  name             = "app-backend"
  forward_protocol = "http"
  forward_port     = 80
}

resource "scaleway_lb_frontend" "app" {
  lb_id        = scaleway_lb.main.id
  name         = "app-frontend"
  inbound_port = 80

  # Wrong: points at the LB's UUID instead of the backend's UUID.
  # Both are valid references that resolve to UUIDs — easy autocomplete mistake
  # when scaleway_lb.main.id and scaleway_lb_backend.app.id are both in scope.
  backend_id = scaleway_lb.main.id

  # The correct reference:
  # backend_id = scaleway_lb_backend.app.id
}

# ── RDB ───────────────────────────────────────────────────────────────────────

resource "scaleway_rdb_instance" "db" {
  name           = "app-db"
  node_type      = "DB-DEV-S"
  engine         = "PostgreSQL-15"
  is_ha_cluster  = false
  disable_backup = true
}

resource "scaleway_rdb_database" "app" {
  instance_id = scaleway_rdb_instance.db.id
  name        = "appdb"
}

resource "scaleway_rdb_user" "app" {
  # Correct: uses a resource reference, so Terraform orders this after the instance.
  instance_id = scaleway_rdb_instance.db.id
  name        = "appuser"
  password    = "S3cr3tP@ssw0rd!"
  is_admin    = false
}

# ── Outputs ───────────────────────────────────────────────────────────────────

output "server_id" {
  value = scaleway_instance_server.app.id
}

output "lb_id" {
  value = scaleway_lb.main.id
}

output "db_instance_id" {
  value = scaleway_rdb_instance.db.id
}
