# Load Balancer with explicit IP allocation.

resource "scaleway_lb_ip" "ip" {}

resource "scaleway_lb" "lb" {
  name  = "lb-with-ip"
  ip_id = scaleway_lb_ip.ip.id
  type  = "LB-S"
}

output "lb_ip" {
  value = scaleway_lb.lb.ip_address
}
