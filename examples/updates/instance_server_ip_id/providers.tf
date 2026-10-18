terraform {
  required_providers {
    scaleway = {
      source = "scaleway/scaleway"
      # The version the stale public_ips drift was found against.
      version = "2.83.0"
    }
  }
}

provider "scaleway" {}
