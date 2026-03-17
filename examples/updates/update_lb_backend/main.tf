# Update scenario: change LB backend health check settings.

variable "check_max_retries" {
  type    = number
  default = 3
}

variable "backend_name" {
  type    = string
  default = "be-v1"
}

resource "scaleway_lb" "lb" {
  name = "lb-for-update-test"
  type = "LB-S"
}

resource "scaleway_lb_backend" "be" {
  lb_id            = scaleway_lb.lb.id
  name             = var.backend_name
  forward_protocol = "http"
  forward_port     = 80

  health_check_max_retries = var.check_max_retries
}

resource "scaleway_lb_frontend" "fe" {
  lb_id        = scaleway_lb.lb.id
  backend_id   = scaleway_lb_backend.be.id
  name         = "fe"
  inbound_port = 80
}
