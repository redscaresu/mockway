# Redis cluster example: single-node cluster.

resource "scaleway_redis_cluster" "redis" {
  name         = "example-redis"
  version      = "7.2.4"
  node_type    = "RED1-MICRO"
  user_name    = "default"
  password     = "R3d1sP@ssw0rd!"
  cluster_size = 1
}

output "cluster_id" {
  value = scaleway_redis_cluster.redis.id
}
