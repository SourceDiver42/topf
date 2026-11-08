apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: block
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: apid
portSelector:
  ports:
    - 50000
  protocol: tcp
ingress:
  - subnet: 0.0.0.0/0
  - subnet: ::/0
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: node-exporter
portSelector:
  ports:
    - 9100
  protocol: tcp
ingress:
{{- range .Nodes }}
{{- if .IP }}
  - subnet: {{ .IP.Prefix .IP.BitLen }}
{{- end }}
{{- end }}
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: kubelet-trustd
portSelector:
  ports:
    - 10250
    - 50001
  protocol: tcp
ingress:
{{- range .Nodes }}
{{- if .IP }}
  - subnet: {{ .IP.Prefix .IP.BitLen }}
{{- end }}
{{- end }}
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: cni-vxlan
portSelector:
  ports:
    - 4789
  protocol: udp
ingress:
{{- range .Nodes }}
{{- if .IP }}
  - subnet: {{ .IP.Prefix .IP.BitLen }}
{{- end }}
{{- end }}
