resource "scaleway_block_volume" "vol" {
  name       = "mockway-block-vol"
  size_in_gb = 20
  iops       = 5000
}
