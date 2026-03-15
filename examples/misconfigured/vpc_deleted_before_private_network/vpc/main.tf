# Workspace 1: owns the VPC.
# Apply this first, then apply ../pn.
# To reproduce the failure: destroy this workspace while ../pn is still applied.

resource "scaleway_vpc" "vpc" {
  name = "example-vpc"
}

output "vpc_id" {
  value = scaleway_vpc.vpc.id
}
