// Copyright 2026 PostFinance AG
// SPDX-License-Identifier: MIT

package omni

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postfinance/topf/internal/topf"
	"go.yaml.in/yaml/v4"
)

const testConfig = `clusterName: omni-test
clusterEndpoint: https://10.5.0.2:6443
kubernetesVersion: 1.35.5
talosVersion: 1.14.0
nodes:
  - host: cp1
    ip: 10.5.0.2
    role: control-plane
    data:
      omniMachineID: id-cp1
  - host: cp2
    ip: 10.5.0.3
    role: control-plane
    data:
      omniMachineID: id-cp2
  - host: worker1
    ip: 10.5.0.5
    role: worker
    data:
      omniMachineID: id-worker1
`

func newRuntime(t *testing.T, config string, files map[string]string) topf.Topf {
	t.Helper()

	dir := t.TempDir()
	files["topf.yaml"] = config

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	rt, err := topf.NewTopfRuntime(topf.RuntimeConfig{
		ConfigPath: filepath.Join(dir, "topf.yaml"),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("creating runtime: %v", err)
	}

	return rt
}

func decode(t *testing.T, data []byte) map[string]map[string]any {
	t.Helper()

	docs := map[string]map[string]any{}
	dec := yaml.NewDecoder(bytes.NewReader(data))

	for {
		var doc map[string]any

		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return docs
		}

		if err != nil {
			t.Fatalf("decoding template: %v", err)
		}

		key := doc["kind"].(string)
		if key == "Machine" {
			key += "/" + doc["name"].(string)
		}

		docs[key] = doc
	}
}

func patchNames(doc map[string]any) []string {
	var names []string

	patches, _ := doc["patches"].([]any)
	for _, p := range patches {
		names = append(names, p.(map[string]any)["name"].(string))
	}

	return names
}

func TestTemplate(t *testing.T) {
	rt := newRuntime(t, testConfig, map[string]string{
		"all/00-install.yaml":            "apiVersion: v1alpha1\nkind: UnattendedInstallConfig\nprovisioning:\n  diskSelector:\n    match: disk.dev_path == \"/dev/vda\"\n",
		"all/10-shared.yaml":             "machine:\n  sysctls:\n    vm.max_map_count: \"262144\"\n",
		"all/20-hostname.yaml.tpl":       "apiVersion: v1alpha1\nkind: HostnameConfig\nauto: \"off\"\nhostname: {{ .Node.Host }}\n",
		"control-plane/10-etcd.yaml.tpl": "machine:\n  nodeLabels:\n    a: b\n---\nmachine:\n  nodeLabels:\n    peers: \"{{ len .Nodes }}\"\n",
		"worker/10-worker.yaml":          "machine:\n  nodeLabels:\n    role: worker\n",
		"node/cp2/10-node.yaml":          "machine:\n  nodeLabels:\n    special: \"true\"\n",
	})

	var buf bytes.Buffer
	if err := Template(rt, &buf); err != nil {
		t.Fatalf("Template() error: %v", err)
	}

	docs := decode(t, buf.Bytes())

	cluster := docs["Cluster"]
	if cluster["name"] != "omni-test" {
		t.Errorf("cluster name: got %v", cluster["name"])
	}

	if v := cluster["talos"].(map[string]any)["version"]; v != "v1.14.0" {
		t.Errorf("talos version: got %v", v)
	}

	want := map[string]string{
		"Cluster":            "all-10-shared",
		"ControlPlane":       "control-plane-10-etcd-1,control-plane-10-etcd-2",
		"Workers":            "worker-10-worker",
		"Machine/id-cp1":     "all-20-hostname",
		"Machine/id-cp2":     "all-20-hostname,node-cp2-10-node",
		"Machine/id-worker1": "all-20-hostname",
	}

	for key, names := range want {
		doc, ok := docs[key]
		if !ok {
			t.Errorf("missing document %s", key)

			continue
		}

		if got := strings.Join(patchNames(doc), ","); got != names {
			t.Errorf("%s patches: expected %s, got %s", key, names, got)
		}
	}

	if got := docs["ControlPlane"]["machines"]; len(got.([]any)) != 2 {
		t.Errorf("control-plane machines: got %v", got)
	}
}

func TestTemplateErrors(t *testing.T) {
	tests := map[string]struct {
		config string
		want   string
	}{
		"missing machine ID": {
			config: strings.Replace(testConfig, "      omniMachineID: id-cp2\n", "      other: x\n", 1),
			want:   "node cp2: data.omniMachineID must be set",
		},
		"duplicate machine ID": {
			config: strings.Replace(testConfig, "id-cp2", "id-cp1", 1),
			want:   "nodes cp1 and cp2 share Omni machine id-cp1",
		},
		"missing talos version": {
			config: strings.Replace(testConfig, "talosVersion: 1.14.0\n", "", 1),
			want:   "talosVersion must be set",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rt := newRuntime(t, tc.config, map[string]string{})

			err := Template(rt, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}
