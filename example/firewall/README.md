# Firewall Example

A five-node cluster (3 control-plane, 2 workers) with a host firewall generated from the node list via `.Nodes`. Ingress is blocked by default; the patches only open what is listed below.

| Port | Protocol | Nodes | Allowed from | Why |
| ------ | ---------- | ------- | -------------- | ----- |
| 50000 | tcp | all | anywhere | Talos API (`talosctl`, `topf`) |
| 9100 | tcp | all | cluster nodes | Prometheus node exporter |
| 10250, 50001 | tcp | all | cluster nodes | kubelet API, trustd — required |
| 4789 | udp | all | cluster nodes | flannel VXLAN — required for the default CNI |
| 443 | tcp | control-plane | anywhere | HTTPS |
| 6443 | tcp | control-plane | anywhere | Kubernetes API — required |
| 2379-2380 | tcp | control-plane | control-plane nodes | etcd — required |

Rules marked *required* are needed for the cluster to function once ingress is blocked. If you run a different CNI or move the Kubernetes API, adjust them accordingly.

Node-sourced rules are rendered from `.Nodes`, so adding a node to `topf.yaml` (or a `nodesProvider`) updates the firewall on every node on the next `topf apply`. Nodes without an `ip` are skipped.

## Layout

```text
.
├── topf.yaml
├── all/
│   ├── 00-install-disk.yaml
│   └── 50-firewall.yaml.tpl      # default block, apid, node exporter, kubelet/trustd, VXLAN
└── control-plane/
    └── 50-firewall.yaml.tpl      # HTTPS, Kubernetes API, etcd
```

## Render

```bash
cd example/firewall
topf render
```

The rendered configs are checked by `TestFirewallExample` in `internal/topf`.

## Caveats

- **Prometheus running in the cluster:** a scrape from a pod on the *same* node arrives with the pod IP as source, not a node IP. Add the pod CIDR (Talos default `10.244.0.0/16`) to the `node-exporter` rule if you scrape from inside the cluster.
- **Lockout risk:** a mistake in the allow rules can make nodes unreachable. `apid` (50000) is open from anywhere here so you can always fix the config with `topf apply`.
