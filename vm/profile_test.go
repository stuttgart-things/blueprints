package main

import (
	"context"
	"strings"
	"testing"
)

func TestParseDotenvNames(t *testing.T) {
	got, err := parseDotenvNames("# comment\nSOPS_AGE_KEY=AGE-SECRET-KEY-1X\n\n  SOPS_GIT_TOKEN = tok\nEMPTY=\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

// Lines the consumer (dagger/ansible) would take literally must not pass.
func TestParseDotenvNamesRejectsWhatTheConsumerTakesLiterally(t *testing.T) {
	for _, tc := range []struct{ line, want string }{
		{"export SOPS_GIT_TOKEN=tok", "export"},
		{`SOPS_GIT_TOKEN="tok"`, "quoted value"},
		{"SOPS_GIT_TOKEN='tok'", "quoted value"},
		{"NOEQUALS", "not NAME=value"},
		{"=value", "not NAME=value"},
	} {
		_, err := parseDotenvNames(tc.line)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: got %v, want error containing %q", tc.line, err, tc.want)
		}
		if err != nil && strings.Contains(err.Error(), "tok") {
			t.Errorf("%q: error leaks the value: %v", tc.line, err)
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
