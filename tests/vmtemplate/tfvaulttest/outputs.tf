
output "vsphere_password" {
  value     = data.vault_kv_secret_v2.myapp.data["password"]
  sensitive = true
}