# RDB instance example: instance + database + user.
#
# mockway translates disable_backup=true to backup_schedule.disabled=true in the
# stored data so that GET /instances/{id} returns the shape the provider reads.
# Without this translation the provider sees drift on every subsequent plan.

resource "scaleway_rdb_instance" "db" {
  name           = "example-db"
  node_type      = "DB-DEV-S"
  engine         = "PostgreSQL-15"
  is_ha_cluster  = false
  disable_backup = true
}

resource "scaleway_rdb_database" "appdb" {
  instance_id = scaleway_rdb_instance.db.id
  name        = "appdb"
}

resource "scaleway_rdb_user" "appuser" {
  instance_id = scaleway_rdb_instance.db.id
  name        = "appuser"
  password    = "S3cr3tP@ssw0rd!"
  is_admin    = false
}

output "instance_id" {
  value = scaleway_rdb_instance.db.id
}
