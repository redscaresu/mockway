# Run this example against a local mockway instance to see the failure:
#
#   go install github.com/redscaresu/mockway/cmd/mockway@latest
#   mockway --port 8080 &
#
#   export SCW_API_URL=http://localhost:8080
#   export SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX
#   export SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000
#   export SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000
#   export SCW_DEFAULT_ORGANIZATION_ID=00000000-0000-0000-0000-000000000000
#   export SCW_DEFAULT_REGION=fr-par
#   export SCW_DEFAULT_ZONE=fr-par-1
#
#   terraform init && terraform apply -auto-approve
#   # Expected: ERROR — cannot delete: dependents exist
#
# Note: if you have a local Scaleway CLI profile (~/.config/scw/config.yaml) you
# will see a "Multiple variable sources detected" warning. This is cosmetic — the
# provider is using the environment variables above, not your real credentials.

terraform {
  required_providers {
    scaleway = {
      source  = "scaleway/scaleway"
      version = "~> 2.40"
    }
  }
}

provider "scaleway" {}
