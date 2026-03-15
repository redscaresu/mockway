# BROKEN: VPC destroyed before its private network, causing a 409.
#
# Two workspaces manage these resources independently. The VPC workspace is
# torn down first while the private network workspace is still applied —
# mockway rejects the VPC deletion because the private network holds a FK
# reference to it.
#
# ── Reproduce ────────────────────────────────────────────────────────────────
#
#   # 1. Apply the VPC workspace
#   cd ../vpc && terraform init && terraform apply -auto-approve
#
#   # 2. Apply this workspace (reads vpc_id from remote state)
#   cd ../pn && terraform init && terraform apply -auto-approve
#
#   # 3. Destroy the VPC workspace while the private network still exists
#   cd ../vpc && terraform destroy -auto-approve
#
#   Error: scaleway-sdk-go: http error 409: conflict: cannot delete: dependents exist
#
# ── Why standard tooling does not catch this ─────────────────────────────────
#
#   terraform validate  ✓ passes
#   terraform plan      ✓ passes — the VPC workspace only sees the VPC
#
# ── Fix ───────────────────────────────────────────────────────────────────────
#
#   Always destroy the private network workspace before the VPC workspace:
#     cd ../pn && terraform destroy -auto-approve
#     cd ../vpc && terraform destroy -auto-approve

data "terraform_remote_state" "vpc" {
  backend = "local"
  config = {
    path = "../vpc/terraform.tfstate"
  }
}

resource "scaleway_vpc_private_network" "pn" {
  name   = "example-pn"
  vpc_id = data.terraform_remote_state.vpc.outputs.vpc_id
}
