resource "scaleway_instance_volume" "vol" {
  name       = "mockway-vol"
  size_in_gb = 20
  type       = "l_ssd"
}
