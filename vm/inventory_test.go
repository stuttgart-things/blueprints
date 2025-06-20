package main

import (
	"testing"
)

func TestParseIPsFromTfOutput(t *testing.T) {
	input := `{
		"ip": {
			"sensitive": false,
			"type": [
				"tuple",
				[
					[
						"tuple",
						[
							"string"
						]
					]
				]
			],
			"value": [
				[
					"10.100.136.144"
				]
			]
		}
	}`

	expected := []string{"10.100.136.144"}
	result, err := ParseIPsFromTfOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != len(expected) {
		t.Fatalf("expected %d IPs, got %d", len(expected), len(result))
	}
	for i := range expected {
		if result[i] != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], result[i])
		}
	}
}
