# DNS zone with a record.

resource "scaleway_domain_zone" "zone" {
  domain    = "example-test.com"
  subdomain = "app"
}

resource "scaleway_domain_record" "a" {
  dns_zone = "${scaleway_domain_zone.zone.subdomain}.${scaleway_domain_zone.zone.domain}"
  name     = "www"
  type     = "A"
  data     = "1.2.3.4"
  ttl      = 3600
}

output "zone_id" {
  value = scaleway_domain_zone.zone.id
}
