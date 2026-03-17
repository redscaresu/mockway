# Load Balancer with a Let's Encrypt certificate.

resource "scaleway_lb" "lb" {
  name = "lb-cert-test"
  type = "LB-S"
}

resource "scaleway_lb_backend" "be" {
  lb_id            = scaleway_lb.lb.id
  name             = "be"
  forward_protocol = "http"
  forward_port     = 80
}

resource "scaleway_lb_frontend" "fe" {
  lb_id        = scaleway_lb.lb.id
  backend_id   = scaleway_lb_backend.be.id
  name         = "fe"
  inbound_port = 443

  certificate_ids = [scaleway_lb_certificate.cert.id]
}

resource "scaleway_lb_certificate" "cert" {
  lb_id = scaleway_lb.lb.id
  name  = "test-cert"

  letsencrypt {
    common_name = "test.example.com"
  }
}

output "cert_id" {
  value = scaleway_lb_certificate.cert.id
}
