apiVersion: v1alpha1
kind: NetworkRuleConfig
name: https
portSelector:
  ports:
    - 443
  protocol: tcp
ingress:
  - subnet: 0.0.0.0/0
  - subnet: ::/0
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: kube-apiserver
portSelector:
  ports:
    - 6443
  protocol: tcp
ingress:
  - subnet: 0.0.0.0/0
  - subnet: ::/0
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: etcd
portSelector:
  ports:
    - 2379-2380
  protocol: tcp
ingress:
{{- range .Nodes }}
{{- if and (eq .Role "control-plane") .IP }}
  - subnet: {{ .IP.Prefix .IP.BitLen }}
{{- end }}
{{- end }}
