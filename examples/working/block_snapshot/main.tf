resource "scaleway_block_volume" "vol" {
  name       = "mockway-snap-source"
  size_in_gb = 20
  iops       = 5000
}

resource "scaleway_block_snapshot" "snap" {
  name      = "mockway-snap"
  volume_id = scaleway_block_volume.vol.id
}
