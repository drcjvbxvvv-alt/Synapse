import * as YAML from 'yaml';
import type {
  LabelItem,
  AnnotationItem,
  RuleItem,
  KubernetesIngressYAML,
} from './ingressTypes';

export function buildIngressYaml(
  values: Record<string, unknown>,
): string {
  const ingressYaml: KubernetesIngressYAML = {
    apiVersion: 'networking.k8s.io/v1',
    kind: 'Ingress',
    metadata: {
      name: values.name as string,
      namespace: values.namespace as string,
      labels: {},
      annotations: {},
    },
    spec: {
      ingressClassName: values.ingressClass as string | undefined,
      rules: [],
      tls: [],
    },
  };

  if (values.labels && Array.isArray(values.labels) && values.labels.length > 0) {
    (values.labels as LabelItem[]).forEach((label) => {
      if (label?.key) {
        ingressYaml.metadata.labels[label.key] = label.value || '';
      }
    });
  }

  if (values.annotations && Array.isArray(values.annotations) && values.annotations.length > 0) {
    (values.annotations as AnnotationItem[]).forEach((annotation) => {
      if (annotation?.key) {
        ingressYaml.metadata.annotations[annotation.key] = annotation.value || '';
      }
    });
  }

  if (values.rules && Array.isArray(values.rules) && values.rules.length > 0) {
    ingressYaml.spec.rules = (values.rules as RuleItem[]).map((rule) => ({
      host: rule.host,
      http: {
        paths: (rule.paths || []).map((path) => ({
          path: path.path,
          pathType: path.pathType,
          backend: {
            service: {
              name: path.serviceName,
              port: {
                number: path.servicePort,
              },
            },
          },
        })),
      },
    }));
  }

  if (values.tls && Array.isArray(values.tls) && values.tls.length > 0) {
    ingressYaml.spec.tls = (values.tls as Array<{ secretName: string; hosts: string[] }>).map((tls) => ({
      secretName: tls.secretName,
      hosts: tls.hosts,
    }));
  }

  return YAML.stringify(ingressYaml);
}
