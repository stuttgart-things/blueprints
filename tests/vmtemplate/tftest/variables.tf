variable "message" {
  description = "Message to print"
  type        = string
  default     = "Hello from null_resource"
}

variable "count_test_enabled" {
  description = "Enable the count test resource"
  type        = bool
  default     = true
}

variable "count_test_instances" {
  description = "Number of count test resources"
  type        = number
  default     = 2
}

variable "for_each_map" {
  description = "Map used for for_each test"
  type        = map(string)
  default     = {
    one = "apple"
    two = "banana"
  }
}
