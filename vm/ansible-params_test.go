package main

import (
	"encoding/json"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestConvertMapToAnsibleParamsKeepsTypes covers #217: nested dicts and lists
// from --parameters-file must reach Ansible as such, not as strings.
func TestConvertMapToAnsibleParamsKeepsTypes(t *testing.T) {
	fileParams := make(map[string]interface{})
	content := `
bin:
  age:
    version: "1.2"
  sops:
    version: "3.9"
packages: [curl, jq]
retries: 3
`
	if err := yaml.Unmarshal([]byte(content), &fileParams); err != nil {
		t.Fatal(err)
	}

	merged := mergeParamMaps(fileParams, parseStringParams("retries=5,user=it's me"))

	got, err := convertMapToAnsibleParams(merged)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("not a JSON object: %v\n%s", err, got)
	}

	want := map[string]interface{}{
		"bin": map[string]interface{}{
			"age":  map[string]interface{}{"version": "1.2"},
			"sops": map[string]interface{}{"version": "3.9"},
		},
		"packages": []interface{}{"curl", "jq"},
		"retries":  float64(5),
		"user":     "it's me",
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Errorf("got %#v, want %#v", decoded, want)
	}
}

func TestConvertMapToAnsibleParamsEmpty(t *testing.T) {
	got, err := convertMapToAnsibleParams(nil)
	if err != nil || got != "" {
		t.Errorf("got %q, %v; want empty", got, err)
	}
}
