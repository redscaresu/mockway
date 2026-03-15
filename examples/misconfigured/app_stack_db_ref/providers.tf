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
#   # Expected: 11 resources created, then ERROR on scaleway_rdb_database.app
#   #           404: not_found — .name was used instead of .id for instance_id

terraform {
  required_providers {
    scaleway = {
      source  = "scaleway/scaleway"
      version = "~> 2.50"
    }
  }
}

provider "scaleway" {}
