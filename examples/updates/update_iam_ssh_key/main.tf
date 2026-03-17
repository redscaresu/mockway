variable "key_name" {
  type = string
}

resource "scaleway_iam_ssh_key" "key" {
  name       = var.key_name
  public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEEYrzDMZVgV5gVA2bBJMdO5VRTaWMXBvPcmLj+lOYdL test@example.com"
}
