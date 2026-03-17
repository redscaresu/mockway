variable "ip_tags" {
  type    = list(string)
  default = []
}

resource "scaleway_instance_ip" "ip" {
  tags = var.ip_tags
}
