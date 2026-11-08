// Copyright 2026 PostFinance AG
// SPDX-License-Identifier: MIT

// Package omni contains the logic for the omni command
package omni

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/postfinance/topf/internal/decryption"
	"github.com/postfinance/topf/internal/topf"
	"github.com/postfinance/topf/pkg/config"
	"go.yaml.in/yaml/v4"
)

var invalidNameChars = regexp.MustCompile(`[^a-z0-9]+`)

func omniManaged(kind string) bool {
	return kind == "UnattendedInstallConfig"
}

const machineIDKey = "omniMachineID"

type versionSpec struct {
	Version string `yaml:"version"`
}

type patch struct {
	Name   string         `yaml:"name"`
	Inline map[string]any `yaml:"inline"`
}

type clusterDoc struct {
	Kind       string      `yaml:"kind"`
	Name       string      `yaml:"name"`
	Kubernetes versionSpec `yaml:"kubernetes"`
	Talos      versionSpec `yaml:"talos"`
	Patches    []patch     `yaml:"patches,omitempty"`
}

type machineSetDoc struct {
	Kind     string   `yaml:"kind"`
	Machines []string `yaml:"machines"`
	Patches  []patch  `yaml:"patches,omitempty"`
}

type machineDoc struct {
	Kind    string  `yaml:"kind"`
	Name    string  `yaml:"name"`
	Patches []patch `yaml:"patches,omitempty"`
}

type renderedNode struct {
	node      *config.Node
	machineID string
	files     []config.RenderedFile
}

// Template writes an Omni cluster template for all configured nodes to w.
func Template(t topf.Topf, w io.Writer) error {
	cfg := t.Config()

	if cfg.TalosVersion == "" {
		return errors.New("talosVersion must be set in topf.yaml for an Omni cluster template")
	}

	if cfg.KubernetesVersion == "" {
		return errors.New("kubernetesVersion must be set in topf.yaml for an Omni cluster template")
	}

	nodes, err := renderNodes(t)
	if err != nil {
		return err
	}

	cluster := clusterDoc{
		Kind:       "Cluster",
		Name:       cfg.ClusterName,
		Kubernetes: versionSpec{Version: "v" + strings.TrimPrefix(cfg.KubernetesVersion, "v")},
		Talos:      versionSpec{Version: "v" + strings.TrimPrefix(cfg.TalosVersion, "v")},
	}

	controlPlane := machineSetDoc{Kind: "ControlPlane", Machines: []string{}}
	workers := machineSetDoc{Kind: "Workers", Machines: []string{}}
	machinePatches := make(map[string][]patch, len(nodes))

	var controlPlaneNodes, workerNodes []renderedNode

	for _, n := range nodes {
		if n.node.Role == config.RoleControlPlane {
			controlPlane.Machines = append(controlPlane.Machines, n.machineID)
			controlPlaneNodes = append(controlPlaneNodes, n)
		} else {
			workers.Machines = append(workers.Machines, n.machineID)
			workerNodes = append(workerNodes, n)
		}
	}

	cluster.Patches, err = place(nodes, config.ScopeAll, machinePatches)
	if err != nil {
		return err
	}

	controlPlane.Patches, err = place(controlPlaneNodes, config.ScopeRole, machinePatches)
	if err != nil {
		return err
	}

	workers.Patches, err = place(workerNodes, config.ScopeRole, machinePatches)
	if err != nil {
		return err
	}

	for _, n := range nodes {
		for _, f := range n.files {
			if f.Scope == config.ScopeNode {
				machinePatches[n.machineID] = append(machinePatches[n.machineID], filePatches(f)...)
			}
		}
	}

	docs := []any{cluster, controlPlane}
	if len(workers.Machines) > 0 {
		docs = append(docs, workers)
	}

	for _, n := range nodes {
		if p := machinePatches[n.machineID]; len(p) > 0 {
			docs = append(docs, machineDoc{Kind: "Machine", Name: n.machineID, Patches: p})
		}
	}

	return encode(w, docs)
}

func renderNodes(t topf.Topf) ([]renderedNode, error) {
	cfg := t.Config()
	decryptCache := decryption.NewCache()
	allNodes := cfg.StaticNodes()
	nodes := make([]renderedNode, 0, len(allNodes))
	seen := map[string]string{}

	for i := range allNodes {
		node := &allNodes[i]

		machineID, ok := node.Data[machineIDKey].(string)
		if !ok || machineID == "" {
			return nil, fmt.Errorf("node %s: data.%s must be set to the Omni machine UUID", node.Host, machineIDKey)
		}

		if other, dup := seen[machineID]; dup {
			return nil, fmt.Errorf("nodes %s and %s share Omni machine %s", other, node.Host, machineID)
		}

		seen[machineID] = node.Host

		patchCtx := &config.PatchContext{
			ClusterName:       cfg.ClusterName,
			ClusterEndpoint:   cfg.ClusterEndpoint.String(),
			KubernetesVersion: cfg.KubernetesVersion,
			TalosVersion:      cfg.TalosVersion,
			SchematicID:       node.SchematicID,
			Data:              cfg.Data,
			Node:              node,
			Nodes:             allNodes,
			PatchesDir:        cfg.PatchesDir,
			DecryptCache:      decryptCache,
		}

		files, secrets, err := patchCtx.RenderFiles()
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", node.Host, err)
		}

		t.AddSecretsToMask(secrets)

		nodes = append(nodes, renderedNode{node: node, machineID: machineID, files: dropManaged(t, node, files)})
	}

	return nodes, nil
}

func dropManaged(t topf.Topf, node *config.Node, files []config.RenderedFile) []config.RenderedFile {
	kept := files[:0]

	for _, f := range files {
		docs := f.Documents[:0]

		for _, doc := range f.Documents {
			if kind, _ := doc["kind"].(string); omniManaged(kind) {
				t.Logger().Warn("skipping document managed by Omni", "node", node.Host, "file", f.Path, "kind", kind)

				continue
			}

			docs = append(docs, doc)
		}

		if len(docs) > 0 {
			f.Documents = docs
			kept = append(kept, f)
		}
	}

	return kept
}

func place(nodes []renderedNode, scope config.PatchScope, machinePatches map[string][]patch) ([]patch, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	var shared []patch

	for _, f := range nodes[0].files {
		if f.Scope != scope {
			continue
		}

		same, err := identicalForAll(nodes, f)
		if err != nil {
			return nil, err
		}

		if same {
			shared = append(shared, filePatches(f)...)

			continue
		}

		for _, n := range nodes {
			for _, nf := range n.files {
				if nf.Scope == scope && nf.Path == f.Path {
					machinePatches[n.machineID] = append(machinePatches[n.machineID], filePatches(nf)...)
				}
			}
		}
	}

	return shared, nil
}

func identicalForAll(nodes []renderedNode, f config.RenderedFile) (bool, error) {
	want, err := yaml.Marshal(f.Documents)
	if err != nil {
		return false, err
	}

	for _, n := range nodes[1:] {
		var got []byte

		for _, nf := range n.files {
			if nf.Scope == f.Scope && nf.Path == f.Path {
				if got, err = yaml.Marshal(nf.Documents); err != nil {
					return false, err
				}
			}
		}

		if !bytes.Equal(want, got) {
			return false, nil
		}
	}

	return true, nil
}

func filePatches(f config.RenderedFile) []patch {
	patches := make([]patch, 0, len(f.Documents))

	for i, doc := range f.Documents {
		name := f.Path
		if len(f.Documents) > 1 {
			name = fmt.Sprintf("%s-%d", f.Path, i+1)
		}

		patches = append(patches, patch{Name: patchName(name), Inline: doc})
	}

	return patches
}

func patchName(path string) string {
	for _, ext := range []string{".tpl", ".yaml", ".yml"} {
		path = strings.ReplaceAll(path, ext, "")
	}

	return strings.Trim(invalidNameChars.ReplaceAllString(strings.ToLower(path), "-"), "-")
}

func encode(w io.Writer, docs []any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)

	for _, doc := range docs {
		if err := enc.Encode(doc); err != nil {
			return err
		}
	}

	return enc.Close()
}
