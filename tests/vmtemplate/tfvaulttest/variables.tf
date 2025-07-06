variable "vault_addr" {
  type        = string
  description = "Vault server address"
}

variable "vault_role_id" {
  type        = string
  description = "AppRole Role ID"
}

variable "vault_secret_id" {
  type        = string
  description = "AppRole Secret ID"
  sensitive   = true
}
