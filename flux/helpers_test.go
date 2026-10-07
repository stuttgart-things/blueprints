package main

import (
	"strings"
	"testing"
)

// TestStampManifestDiffersPerCall covers #182: two applies of the same
// manifest must not be byte-identical, or the second is served from cache.
func TestStampManifestDiffersPerCall(t *testing.T) {
	content := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: flux-system\n"

	a := stampManifest(content)
	b := stampManifest(content)

	if a == b {
		t.Fatalf("two stamps of the same manifest are identical:\n%s", a)
	}
	for _, got := range []string{a, b} {
		if !strings.HasPrefix(got, content) {
			t.Errorf("manifest content changed:\n%s", got)
		}
		if !strings.HasSuffix(got, "\n") || strings.Count(got, "# flux bootstrap run: ") != 1 {
			t.Errorf("expected exactly one trailing stamp comment:\n%s", got)
		}
	}
}
