# RDB instance with ACL rules.

resource "scaleway_rdb_instance" "db" {
  name           = "rdb-acl-test"
  node_type      = "DB-DEV-S"
  engine         = "PostgreSQL-15"
  is_ha_cluster  = false
  disable_backup = true
}

resource "scaleway_rdb_acl" "acl" {
  instance_id = scaleway_rdb_instance.db.id

  acl_rules {
    ip          = "1.2.3.4/32"
    description = "office"
  }

  acl_rules {
    ip          = "5.6.7.8/32"
    description = "vpn"
  }
}

output "acl_rules" {
  value = scaleway_rdb_acl.acl.acl_rules
}
