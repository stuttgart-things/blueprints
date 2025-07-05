output "count_ids" {
  value = [for r in null_resource.count_test : r.id]
  description = "IDs from count_test resources"
}

output "foreach_keys" {
  value = [for r in null_resource.for_each_test : r.triggers.key]
  description = "Keys from for_each_test"
}
