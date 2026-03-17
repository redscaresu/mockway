resource "scaleway_rdb_instance" "db" {
  name           = "mockway-db"
  node_type      = "DB-DEV-S"
  engine         = "PostgreSQL-15"
  is_ha_cluster  = false
  disable_backup = true
}

resource "scaleway_rdb_read_replica" "replica" {
  instance_id = scaleway_rdb_instance.db.id
  same_zone   = true
}
