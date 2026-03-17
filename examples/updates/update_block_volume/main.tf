variable "vol_name" {
  type = string
}

variable "vol_tags" {
  type    = list(string)
  default = []
}

resource "scaleway_block_volume" "vol" {
  name       = var.vol_name
  iops       = 5000
  size_in_gb = 20
  tags       = var.vol_tags
}
