# See ../platform/main.tf for the full reproduction steps.
#
#   export SCW_API_URL=http://localhost:8080
#   export SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX
#   export SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000
#   export SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000
#   export SCW_DEFAULT_ORGANIZATION_ID=00000000-0000-0000-0000-000000000000
#   export SCW_DEFAULT_REGION=fr-par
#   export SCW_DEFAULT_ZONE=fr-par-1

terraform {
  required_providers {
    scaleway = {
      source  = "scaleway/scaleway"
      version = "~> 2.50"
    }
  }
}

provider "scaleway" {}
