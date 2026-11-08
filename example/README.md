# TOPF Example Cluster

This directory contains a complete example TOPF cluster configuration. It demonstrates a multi-node Talos cluster with layered patches, templates, and per-node overrides.

## Layout

```text
.
├── topf.yaml                     # Main cluster configuration
├── manifest.yaml.tpl             # Schematic definition (referenced by schematicId in topf.yaml)
├── all/                          # Patches applied to every node (v1.14 multi-doc)
│   ├── 00-install-disk.yaml      # Install disk (UnattendedInstallConfig)
│   ├── 01-base.yaml.tpl          # Kubelet + node labels (KubeletConfig, KubeNodeConfig)
│   ├── 05-hostname.yaml.tpl      # Use nodes[].host as Talos hostname (HostnameConfig)
│   ├── 09-logging.yaml.tpl       # Optional remote logging (env-driven)
│   ├── 10-registry.yaml.tpl      # Optional registry mirror (env-driven)
│   └── 20-firewall.yaml.tpl      # Block ingress, allow apid and kubelet/trustd
├── control-plane/                # Patches applied to control-plane nodes
│   ├── 01-base.yaml              # Talos API access (KubeTalosAPIAccessConfig) + disable discovery
│   └── 20-firewall.yaml.tpl      # Allow Kubernetes API and etcd
├── node/                         # Per-node patches
│   └── node5/
│       └── 00-local-storage.yaml.tpl  # UserVolumeConfig for node5's storage disk
```

## Important: hostnames

The `host` value in `topf.yaml` is used by TOPF for display, logging, and node selection. It is **not** automatically applied as the Talos hostname. Talos defaults to auto-generated hostnames such as `talos-XXX-XXX` unless you override them.

The patch `all/05-hostname.yaml.tpl` sets the Talos hostname from `nodes[].host`:

```yaml
apiVersion: v1alpha1
kind: HostnameConfig
auto: "off"
hostname: {{ .Node.Host }}
```

If you want Kubernetes node names to match the `host` values, keep this patch in your configuration.

## Firewall

The `20-firewall.yaml.tpl` patches block all ingress except the Talos API, the Kubernetes API, kubelet/trustd between nodes and etcd between control-plane nodes. The node IPs come from `.Nodes`, so the rules follow the node list. Add rules for your CNI before applying.

## Render the example

To render machine configs without applying them:

```bash
cd example/
topf render
```

The generated files in `output/` are ignored by git. Do not commit them.

## Apply the example

This example uses placeholder IPs and secrets. Before applying, update `topf.yaml`, the patches, and generate real secrets with `topf secrets`.
