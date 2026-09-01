package main

import (
	"context"
	"reflect"
	"testing"
)

func TestParseVmiAddress(t *testing.T) {
	tests := []struct {
		name      string
		out       string
		wantPhase string
		wantIP    string
	}{
		{
			// The kubernetes module merges stderr into stdout and never fails
			// the exec, so this is what a poll sees before the VMI exists.
			name: "vmi not found yet",
			out:  `Error from server (NotFound): virtualmachineinstances.kubevirt.io "dev5" not found`,
		},
		{
			name:      "running but agent has not reported an address",
			out:       `{"status":{"phase":"Running","interfaces":[]}}`,
			wantPhase: "Running",
		},
		{
			name:      "scheduling",
			out:       `{"status":{"phase":"Scheduling"}}`,
			wantPhase: "Scheduling",
		},
		{
			name:      "running with address",
			out:       `{"status":{"phase":"Running","interfaces":[{"name":"default","ipAddress":"10.31.103.58"}]}}`,
			wantPhase: "Running",
			wantIP:    "10.31.103.58",
		},
		{
			// An interface can be listed before the agent has an address for
			// it; take the first one that actually carries an address.
			name:      "first interface has no address",
			out:       `{"status":{"phase":"Running","interfaces":[{"name":"default"},{"name":"net1","ipAddress":"10.31.103.59"}]}}`,
			wantPhase: "Running",
			wantIP:    "10.31.103.59",
		},
		{
			name:      "json preceded by a warning line",
			out:       "W0101 12:00:00.000000   1 warnings.go:70] some deprecation notice\n{\"status\":{\"phase\":\"Running\",\"interfaces\":[{\"ipAddress\":\"10.31.103.60\"}]}}",
			wantPhase: "Running",
			wantIP:    "10.31.103.60",
		},
		{
			name: "truncated json",
			out:  `{"status":{"phase":"Run`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase, ip := parseVmiAddress(tt.out)
			if phase != tt.wantPhase {
				t.Errorf("phase = %q, want %q", phase, tt.wantPhase)
			}
			if ip != tt.wantIP {
				t.Errorf("ip = %q, want %q", ip, tt.wantIP)
			}
		})
	}
}

func TestAppendKclParameters(t *testing.T) {
	tests := []struct {
		name       string
		parameters string
		extra      []string
		want       string
	}{
		{
			name:  "no caller parameters",
			extra: []string{"vmName=dev5", "namespace=vms"},
			want:  "vmName=dev5,namespace=vms",
		},
		{
			// The forced values must come last: KCL applies -D left to right,
			// so the rightmost occurrence of a key wins.
			name:       "forced values win over caller parameters",
			parameters: "vmName=stale,memory=8Gi",
			extra:      []string{"vmName=dev5", "namespace=vms"},
			want:       "vmName=stale,memory=8Gi,vmName=dev5,namespace=vms",
		},
		{
			name:       "blank caller parameters are dropped",
			parameters: "   ",
			extra:      []string{"vmName=dev5"},
			want:       "vmName=dev5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := appendKclParameters(tt.parameters, tt.extra...); got != tt.want {
				t.Errorf("appendKclParameters() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSimpleInventory(t *testing.T) {
	if got, want := simpleInventory("10.31.103.58"), "[all]\n10.31.103.58\n"; got != want {
		t.Errorf("simpleInventory() = %q, want %q", got, want)
	}
}

func TestHarvesterNameDefaults(t *testing.T) {
	tests := []struct {
		name             string
		inlineParameters string
		want             []string
	}{
		{
			// Without these the KCL module falls back to its own fixed
			// "dev2-disk-0" / "dev4", which two bootstrap runs would share.
			name: "nothing set derives both names from the vm",
			want: []string{"pvcName=bootstrap-disk-0", "secretName=bootstrap-cloud-init"},
		},
		{
			name:             "an explicit pvc is left alone",
			inlineParameters: "pvcName=restored-disk-0,memory=16Gi",
			want:             []string{"secretName=bootstrap-cloud-init"},
		},
		{
			name:             "both set derives nothing",
			inlineParameters: "pvcName=restored-disk-0,secretName=restored-cloud-init",
			want:             nil,
		},
	}

	m := &Vm{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.harvesterNameDefaults(context.Background(), nil, tt.inlineParameters, "bootstrap")
			if err != nil {
				t.Fatalf("harvesterNameDefaults() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("harvesterNameDefaults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveVmName(t *testing.T) {
	tests := []struct {
		name             string
		inlineParameters string
		want             string
	}{
		{
			name: "no parameters at all",
		},
		{
			name:             "named inline",
			inlineParameters: "vmName=bootstrap,namespace=vms",
			want:             "bootstrap",
		},
		{
			// BakeHarvester forces its own vmName in last; -D is applied left
			// to right, so the last one is the one KCL renders with.
			name:             "the last inline vmName wins",
			inlineParameters: "vmName=caller,namespace=vms,vmName=forced",
			want:             "forced",
		},
		{
			name:             "a blank vmName is not a name",
			inlineParameters: "vmName= ,namespace=vms",
		},
		{
			name:             "other parameters only",
			inlineParameters: "namespace=vms,memory=16Gi",
		},
	}

	m := &Vm{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// paramsFile is nil: reading one needs an engine session, and the
			// file branch is covered by the render call in the module's docs.
			got, err := m.resolveVmName(context.Background(), nil, tt.inlineParameters)
			if err != nil {
				t.Fatalf("resolveVmName() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveVmName() = %q, want %q", got, tt.want)
			}
		})
	}
}
