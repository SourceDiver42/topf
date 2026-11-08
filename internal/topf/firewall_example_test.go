// Copyright 2026 PostFinance AG
// SPDX-License-Identifier: MIT

package topf

import (
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/postfinance/topf/pkg/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

func TestFirewallExample(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("../../example/firewall")); err != nil {
		t.Fatalf("copying example: %v", err)
	}

	rt, err := NewTopfRuntime(RuntimeConfig{
		ConfigPath: filepath.Join(dir, "topf.yaml"),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("creating runtime: %v", err)
	}

	nodes, err := rt.Render(t.Context(), false)
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}

	allNodes := []string{"10.5.0.2/32", "10.5.0.3/32", "10.5.0.4/32", "10.5.0.5/32", "10.5.0.6/32"}
	controlPlanes := allNodes[:3]
	anywhere := []string{"0.0.0.0/0", "::/0"}

	common := map[string][]string{
		"apid":           anywhere,
		"node-exporter":  allNodes,
		"kubelet-trustd": allNodes,
		"cni-vxlan":      allNodes,
	}

	controlPlaneOnly := map[string][]string{
		"https":          anywhere,
		"kube-apiserver": anywhere,
		"etcd":           controlPlanes,
	}

	for _, node := range nodes {
		t.Run(node.Node.Host, func(t *testing.T) {
			if node.Error != nil {
				t.Fatalf("node error: %v", node.Error)
			}

			want := maps.Clone(common)
			if node.Node.Role == config.RoleControlPlane {
				maps.Copy(want, controlPlaneOnly)
			}

			got := map[string][]string{}
			blocked := false

			for _, doc := range node.ConfigProvider().Documents() {
				switch d := doc.(type) {
				case *network.DefaultActionConfigV1Alpha1:
					blocked = d.Ingress == nethelpers.DefaultActionBlock
				case *network.RuleConfigV1Alpha1:
					for _, rule := range d.Ingress {
						got[d.MetaName] = append(got[d.MetaName], rule.Subnet.String())
					}
				}
			}

			if !blocked {
				t.Error("expected default ingress action to be block")
			}

			for name, subnets := range want {
				if !slices.Equal(got[name], subnets) {
					t.Errorf("rule %s: expected %v, got %v", name, subnets, got[name])
				}
			}

			for name := range got {
				if _, ok := want[name]; !ok {
					t.Errorf("unexpected rule %s", name)
				}
			}
		})
	}

	if len(nodes) != 5 {
		t.Errorf("expected 5 rendered nodes, got %d", len(nodes))
	}
}
