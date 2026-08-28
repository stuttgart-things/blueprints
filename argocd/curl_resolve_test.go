package main

import "testing"

// The pin exists because a Dagger container resolves through the runner VM's
// resolver, not the job pod's -- and the lab recursor answers the lab zone
// only intermittently. A wrong flag here is silent: curl simply resolves
// normally again and the failure comes back under load.
func TestCurlResolveFlag(t *testing.T) {
	cases := []struct {
		name, addr, resolve, want string
	}{
		{"empty pin stays on DNS", "https://bao.lab", "", ""},
		{"host:ip fills in 443 for https", "https://bao.lab", "bao.lab:10.0.0.1",
			" --resolve bao.lab:443:10.0.0.1"},
		{"host:ip fills in 80 for http", "http://bao.lab", "bao.lab:10.0.0.1",
			" --resolve bao.lab:80:10.0.0.1"},
		{"explicit port in the url wins", "https://vault.lab:8200", "vault.lab:10.0.0.2",
			" --resolve vault.lab:8200:10.0.0.2"},
		{"a path does not become the port", "https://bao.lab/v1", "bao.lab:10.0.0.1",
			" --resolve bao.lab:443:10.0.0.1"},
		{"full curl form passes through", "https://bao.lab", "bao.lab:8200:10.0.0.3",
			" --resolve bao.lab:8200:10.0.0.3"},
		{"half a pin is no pin", "https://bao.lab", "bao.lab", ""},
		{"missing ip is no pin", "https://bao.lab", "bao.lab:", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := curlResolveFlag(c.addr, c.resolve); got != c.want {
				t.Fatalf("curlResolveFlag(%q, %q) = %q, want %q", c.addr, c.resolve, got, c.want)
			}
		})
	}
}
