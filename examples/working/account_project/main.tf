# Self-managed project lifecycle.
#
# This is the shape infrafactory Layer 3 generates (ADR-0010): the stack
# creates its own project and wires every resource to it, so the run's
# entire blast radius is one project that can be created and destroyed
# as part of the normal IaC lifecycle.
#
# The destroy ordering matters and is enforced: mockway refuses to
# delete a project that still owns resources, the same way real Scaleway
# does. Terraform's dependency graph destroys the VPC before the project
# because of the project_id reference below.

resource "scaleway_account_project" "run" {
  name        = "infrafactory-example"
  description = "Self-managed project for a single run"
}

resource "scaleway_vpc" "main" {
  name       = "example-vpc"
  project_id = scaleway_account_project.run.id
}

output "project_id" {
  value = scaleway_account_project.run.id
}
