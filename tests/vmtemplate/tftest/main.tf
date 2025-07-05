terraform {
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "~> 3.2"
    }
  }
}

provider "null" {}

# Single resource with a message
resource "null_resource" "message_test" {
  triggers = {
    msg = var.message
  }

  provisioner "local-exec" {
    command = "echo ${self.triggers.msg}"
  }
}

# Count-based resource (conditional)
resource "null_resource" "count_test" {
  count = var.count_test_enabled ? var.count_test_instances : 0

  triggers = {
    index = count.index
  }

  provisioner "local-exec" {
    command = "echo Count index: ${count.index}"
  }
}

# for_each test with map variable
resource "null_resource" "for_each_test" {
  for_each = var.for_each_map

  triggers = {
    key   = each.key
    value = each.value
  }

  provisioner "local-exec" {
    command = "echo For_each key: ${each.key}, value: ${each.value}"
  }
}
