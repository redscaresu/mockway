variable "cluster_name" {
  type = string
}

variable "tags" {
  type    = list(string)
  default = []
}

resource "scaleway_redis_cluster" "redis" {
  name      = var.cluster_name
  version   = "7.0.12"
  node_type = "RED1-MICRO"
  tags      = var.tags

  user_name = "admin"
  password  = "Str0ngP@ssw0rd!"
}
