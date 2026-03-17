# Update scenario: change RDB instance name and backup settings.

variable "db_name" {
  type = string
}

variable "disable_backup" {
  type = bool
}

resource "scaleway_rdb_instance" "db" {
  name           = var.db_name
  node_type      = "DB-DEV-S"
  engine         = "PostgreSQL-15"
  is_ha_cluster  = false
  disable_backup = var.disable_backup
}
