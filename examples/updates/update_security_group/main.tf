variable "sg_name" {
  type = string
}

variable "inbound_default" {
  type    = string
  default = "drop"
}

resource "scaleway_instance_security_group" "sg" {
  name                    = var.sg_name
  inbound_default_policy  = var.inbound_default
  outbound_default_policy = "accept"
}
