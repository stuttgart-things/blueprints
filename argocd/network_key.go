package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"dagger/argocd/internal/dagger"
)

// DetectNetworkKey runs `kubectl get nodes -o json` against the target
// cluster and returns the /24 prefix shared by the nodes' InternalIP
// addresses (e.g. "10.31.102"). This is the format expected by
// render-clusterbook-cluster-config / bootstrap-clusterbook-cluster as
// --network-key.
//
// When nodes span multiple /24 subnets (rare but possible with mixed
// pools), the most frequently observed prefix wins. IPv6 addresses are
// ignored.
func (m *Argocd) DetectNetworkKey(
	ctx context.Context,
	// Kubeconfig secret for cluster access
	kubeConfig *dagger.Secret,
) (string, error) {
	if kubeConfig == nil {
		return "", fmt.Errorf("kube-config is required")
	}

	out, err := dag.Kubernetes().Command(
		ctx,
		dagger.KubernetesCommandOpts{
			Operation:    "get",
			ResourceKind: "nodes -o json",
			KubeConfig:   kubeConfig,
		},
	)
	if err != nil {
		return "", fmt.Errorf("kubectl get nodes: %w", err)
	}

	// kubectl output is wrapped through `(... 2>&1) || true` upstream, so
	// stderr noise can prefix the JSON. Locate the first '{' to be robust.
	jsonStart := strings.Index(out, "{")
	if jsonStart < 0 {
		return "", fmt.Errorf("no JSON in kubectl output: %s", strings.TrimSpace(out))
	}

	var payload struct {
		Items []struct {
			Status struct {
				Addresses []struct {
					Type    string `json:"type"`
					Address string `json:"address"`
				} `json:"addresses"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out[jsonStart:]), &payload); err != nil {
		return "", fmt.Errorf("parse kubectl output: %w", err)
	}

	counts := map[string]int{}
	for _, n := range payload.Items {
		for _, a := range n.Status.Addresses {
			if a.Type != "InternalIP" {
				continue
			}
			ip := net.ParseIP(a.Address)
			if ip == nil {
				continue
			}
			v4 := ip.To4()
			if v4 == nil {
				continue
			}
			counts[fmt.Sprintf("%d.%d.%d", v4[0], v4[1], v4[2])]++
		}
	}
	if len(counts) == 0 {
		return "", fmt.Errorf("no IPv4 InternalIP addresses found on cluster nodes")
	}

	var (
		bestKey string
		bestN   int
	)
	for k, n := range counts {
		if n > bestN {
			bestKey, bestN = k, n
		}
	}
	return bestKey, nil
}
