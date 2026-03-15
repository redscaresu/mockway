# Container Registry namespace example.

resource "scaleway_registry_namespace" "ns" {
  name      = "example-registry"
  is_public = false
}

output "namespace_id" {
  value = scaleway_registry_namespace.ns.id
}

output "endpoint" {
  value = scaleway_registry_namespace.ns.endpoint
}
