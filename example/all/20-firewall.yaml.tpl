---
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
    - subnet: {{ .IP }}/32
{{- end }}
{{- end }}
