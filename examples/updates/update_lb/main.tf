variable "lb_name" {
  type = string
}

variable "lb_description" {
  type    = string
  default = ""
}

resource "scaleway_lb" "lb" {
  name        = var.lb_name
  description = var.lb_description
  type        = "LB-S"
}
