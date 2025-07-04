package main

import (
	"regexp"
)

// ExtractTemplateName returns the vsphere artifact name (e.g., ubuntu24-20250704) from the given log string
func ExtractTemplateName(log string) string {
	re := regexp.MustCompile(`artifact \[\]string\{"0", "id", "([^"]+)"\}`)
	match := re.FindStringSubmatch(log)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}
