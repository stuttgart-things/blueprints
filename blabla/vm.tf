module "blabla" {
  source   = "github.com/stuttgart-things/vsphere-vm?ref=v1.7.5-2.7.0"
  vm_count = 1
  vsphere_vm_name = "blabla"
  vm_memory = 8192
  vsphere_vm_template = "sthings-u24"
  vm_disk_size = "64"
  vm_num_cpus = 6
  firmware = "bios"
  vsphere_vm_folder_path = "stuttgart-things/testing"
  vsphere_datacenter = "/LabUL"
  vsphere_datastore = "/LabUL/datastore/ESX01-Local1"
  vsphere_resource_pool = "/LabUL/host/Cluster-V6.7/Resources"
  vsphere_network = "/LabUL/network/MGMT-10.31.101"
  bootstrap = ["echo STUTTGART-THINGS"]
  annotation = "VSPHERE-VM blabla sthings-u24 BUILD w/ TERRAFORM FOR STUTTGART-THINGS"
  vsphere_server   = data.vault_kv_secret_v2.vsphere.data["ip"]
  vsphere_user     = data.vault_kv_secret_v2.vsphere.data["username"]
  vsphere_password = data.vault_kv_secret_v2.vsphere.data["password"]
  vm_ssh_user      = data.vault_kv_secret_v2.vsphere.data["vm_ssh_user"]
  vm_ssh_password  = data.vault_kv_secret_v2.vsphere.data["vm_ssh_password"]
}

output "ip" {
  value = [module.blabla.ip]
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

data "vault_kv_secret_v2" "vsphere" {
  mount = "cloud"
  name  = "vsphere-labul"
}

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
