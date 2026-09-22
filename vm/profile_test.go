package main

import (
	"context"
	"testing"
)

func TestParseDotenvNames(t *testing.T) {
	got := parseDotenvNames("# comment\nSOPS_AGE_KEY=AGE-SECRET-KEY-1X\n\nexport SOPS_GIT_TOKEN=\"tok\"\nEMPTY=\nNOEQUALS\n")
	want := map[string]bool{"SOPS_AGE_KEY": true, "SOPS_GIT_TOKEN": true, "EMPTY": false}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %v, want %v", k, got[k], v)
		}
	}
}

func TestCheckAnsibleEnvNothingRequired(t *testing.T) {
	if err := checkAnsibleEnv(context.Background(), nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckAnsibleEnvMissingSecret(t *testing.T) {
	err := checkAnsibleEnv(context.Background(), []string{"SOPS_AGE_KEY", "SOPS_GIT_TOKEN"}, nil)
	if err == nil || err.Error() != "profile requires ansibleEnv SOPS_AGE_KEY, SOPS_GIT_TOKEN, not set in --env-secrets" {
		t.Fatalf("unexpected error: %v", err)
	}
}
