terraform {
  required_providers {
    vault = {
      source  = "hashicorp/vault"
      version = "~> 5.0"
    }
  }
}

provider "vault" {
  address = var.vault_addr

  auth_login {
    path = "auth/approle/login"
    parameters = {
      role_id   = var.vault_role_id
      secret_id = var.vault_secret_id
    }
  }
}

# For KV v2, include `/data/` in the path
data "vault_kv_secret_v2" "myapp" {
  mount = "cloud"
  name  = "vsphere"
}
