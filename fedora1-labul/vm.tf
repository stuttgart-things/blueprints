module "fedora1" {
  source   = "github.com/stuttgart-things/vsphere-vm?ref=v1.7.5-2.7.0"
  vm_count = 1
  vsphere_vm_name = "fedora1"
  vm_memory = 8192
  vsphere_vm_template = "sthings-fedora43"
  vm_disk_size = "96"
  vm_num_cpus = 8
  firmware = "bios"
  vsphere_vm_folder_path = "stuttgart-things/testing"
  vsphere_datacenter = "/LabUL"
  vsphere_datastore = "/LabUL/datastore/UL-ESX-SATA-10"
  vsphere_resource_pool = "/LabUL/host/Cluster-V6.7/Resources"
  vsphere_network = "/LabUL/network/LAB-10.31.103"
  bootstrap = ["echo STUTTGART-THINGS"]
  annotation = "VSPHERE-VM fedora1 sthings-fedora43 BUILD w/ TERRAFORM FOR STUTTGART-THINGS"
  vsphere_server   = data.vault_kv_secret_v2.vsphere.data["ip"]
  vsphere_user     = data.vault_kv_secret_v2.vsphere.data["username"]
  vsphere_password = data.vault_kv_secret_v2.vsphere.data["password"]
  vm_ssh_user      = data.vault_kv_secret_v2.vsphere.data["vm_ssh_user"]
  vm_ssh_password  = data.vault_kv_secret_v2.vsphere.data["vm_ssh_password"]
}

output "ip" {
  value = [module.fedora1.ip]
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
