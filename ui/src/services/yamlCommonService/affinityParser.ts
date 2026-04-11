import type { SchedulingConfig } from '../../types/workload';
import { parseCommaString } from './stringHelpers';

// ==================== 親和性解析 ====================

export const parseAffinityToScheduling = (affinity: Record<string, unknown> | undefined): Record<string, unknown> | undefined => {
  if (!affinity) return undefined;

  const scheduling: Record<string, unknown> = {};

  const nodeAffinity = affinity.nodeAffinity as Record<string, unknown> | undefined;
  if (nodeAffinity) {
    const required = nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution as Record<string, unknown> | undefined;
    if (required?.nodeSelectorTerms) {
      const terms = required.nodeSelectorTerms as Array<Record<string, unknown>>;
      const nodeAffinityRequired: Array<{ key: string; operator: string; values: string }> = [];

      terms.forEach(term => {
        const matchExpressions = term.matchExpressions as Array<{ key: string; operator: string; values?: string[] }> | undefined;
        if (matchExpressions) {
          matchExpressions.forEach(expr => {
            nodeAffinityRequired.push({
              key: expr.key,
              operator: expr.operator,
              values: (expr.values || []).join(', '),
            });
          });
        }
      });

      if (nodeAffinityRequired.length > 0) {
        scheduling.nodeAffinityRequired = nodeAffinityRequired;
      }
    }

    const preferred = nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution as Array<Record<string, unknown>> | undefined;
    if (preferred && preferred.length > 0) {
      const nodeAffinityPreferred: Array<{ weight: number; key: string; operator: string; values: string }> = [];

      preferred.forEach(pref => {
        const weight = pref.weight as number;
        const preference = pref.preference as Record<string, unknown>;
        const matchExpressions = preference?.matchExpressions as Array<{ key: string; operator: string; values?: string[] }> | undefined;

        if (matchExpressions) {
          matchExpressions.forEach(expr => {
            nodeAffinityPreferred.push({
              weight,
              key: expr.key,
              operator: expr.operator,
              values: (expr.values || []).join(', '),
            });
          });
        }
      });

      if (nodeAffinityPreferred.length > 0) {
        scheduling.nodeAffinityPreferred = nodeAffinityPreferred;
      }
    }
  }

  const podAffinity = affinity.podAffinity as Record<string, unknown> | undefined;
  if (podAffinity) {
    const required = podAffinity.requiredDuringSchedulingIgnoredDuringExecution as Array<Record<string, unknown>> | undefined;
    if (required && required.length > 0) {
      const podAffinityRequired: Array<{ topologyKey: string; labelKey: string; operator: string; labelValues: string }> = [];

      required.forEach(term => {
        const topologyKey = term.topologyKey as string;
        const labelSelector = term.labelSelector as Record<string, unknown> | undefined;
        const matchExpressions = labelSelector?.matchExpressions as Array<{ key: string; operator: string; values?: string[] }> | undefined;

        if (matchExpressions) {
          matchExpressions.forEach(expr => {
            podAffinityRequired.push({
              topologyKey,
              labelKey: expr.key,
              operator: expr.operator,
              labelValues: (expr.values || []).join(', '),
            });
          });
        }

        const matchLabels = labelSelector?.matchLabels as Record<string, string> | undefined;
        if (matchLabels) {
          Object.entries(matchLabels).forEach(([key, value]) => {
            podAffinityRequired.push({
              topologyKey,
              labelKey: key,
              operator: 'In',
              labelValues: value,
            });
          });
        }
      });

      if (podAffinityRequired.length > 0) {
        scheduling.podAffinityRequired = podAffinityRequired;
      }
    }

    const preferred = podAffinity.preferredDuringSchedulingIgnoredDuringExecution as Array<Record<string, unknown>> | undefined;
    if (preferred && preferred.length > 0) {
      const podAffinityPreferred: Array<{ weight: number; topologyKey: string; labelKey: string; operator: string; labelValues: string }> = [];

      preferred.forEach(pref => {
        const weight = pref.weight as number;
        const podAffinityTerm = pref.podAffinityTerm as Record<string, unknown>;
        const topologyKey = podAffinityTerm?.topologyKey as string;
        const labelSelector = podAffinityTerm?.labelSelector as Record<string, unknown> | undefined;
        const matchExpressions = labelSelector?.matchExpressions as Array<{ key: string; operator: string; values?: string[] }> | undefined;

        if (matchExpressions) {
          matchExpressions.forEach(expr => {
            podAffinityPreferred.push({
              weight,
              topologyKey,
              labelKey: expr.key,
              operator: expr.operator,
              labelValues: (expr.values || []).join(', '),
            });
          });
        }

        const matchLabels = labelSelector?.matchLabels as Record<string, string> | undefined;
        if (matchLabels) {
          Object.entries(matchLabels).forEach(([key, value]) => {
            podAffinityPreferred.push({
              weight,
              topologyKey,
              labelKey: key,
              operator: 'In',
              labelValues: value,
            });
          });
        }
      });

      if (podAffinityPreferred.length > 0) {
        scheduling.podAffinityPreferred = podAffinityPreferred;
      }
    }
  }

  const podAntiAffinity = affinity.podAntiAffinity as Record<string, unknown> | undefined;
  if (podAntiAffinity) {
    const required = podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution as Array<Record<string, unknown>> | undefined;
    if (required && required.length > 0) {
      const podAntiAffinityRequired: Array<{ topologyKey: string; labelKey: string; operator: string; labelValues: string }> = [];

      required.forEach(term => {
        const topologyKey = term.topologyKey as string;
        const labelSelector = term.labelSelector as Record<string, unknown> | undefined;
        const matchExpressions = labelSelector?.matchExpressions as Array<{ key: string; operator: string; values?: string[] }> | undefined;

        if (matchExpressions) {
          matchExpressions.forEach(expr => {
            podAntiAffinityRequired.push({
              topologyKey,
              labelKey: expr.key,
              operator: expr.operator,
              labelValues: (expr.values || []).join(', '),
            });
          });
        }

        const matchLabels = labelSelector?.matchLabels as Record<string, string> | undefined;
        if (matchLabels) {
          Object.entries(matchLabels).forEach(([key, value]) => {
            podAntiAffinityRequired.push({
              topologyKey,
              labelKey: key,
              operator: 'In',
              labelValues: value,
            });
          });
        }
      });

      if (podAntiAffinityRequired.length > 0) {
        scheduling.podAntiAffinityRequired = podAntiAffinityRequired;
      }
    }

    const preferred = podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution as Array<Record<string, unknown>> | undefined;
    if (preferred && preferred.length > 0) {
      const podAntiAffinityPreferred: Array<{ weight: number; topologyKey: string; labelKey: string; operator: string; labelValues: string }> = [];

      preferred.forEach(pref => {
        const weight = pref.weight as number;
        const podAffinityTerm = pref.podAffinityTerm as Record<string, unknown>;
        const topologyKey = podAffinityTerm?.topologyKey as string;
        const labelSelector = podAffinityTerm?.labelSelector as Record<string, unknown> | undefined;
        const matchExpressions = labelSelector?.matchExpressions as Array<{ key: string; operator: string; values?: string[] }> | undefined;

        if (matchExpressions) {
          matchExpressions.forEach(expr => {
            podAntiAffinityPreferred.push({
              weight,
              topologyKey,
              labelKey: expr.key,
              operator: expr.operator,
              labelValues: (expr.values || []).join(', '),
            });
          });
        }

        const matchLabels = labelSelector?.matchLabels as Record<string, string> | undefined;
        if (matchLabels) {
          Object.entries(matchLabels).forEach(([key, value]) => {
            podAntiAffinityPreferred.push({
              weight,
              topologyKey,
              labelKey: key,
              operator: 'In',
              labelValues: value,
            });
          });
        }
      });

      if (podAntiAffinityPreferred.length > 0) {
        scheduling.podAntiAffinityPreferred = podAntiAffinityPreferred;
      }
    }
  }

  return Object.keys(scheduling).length > 0 ? scheduling : undefined;
};

// ==================== 表單排程配置構建 ====================

export const buildSchedulingFromForm = (formData: Record<string, unknown>): SchedulingConfig | undefined => {
  const scheduling = formData.scheduling as Record<string, unknown> | undefined;
  if (!scheduling) return undefined;

  const result: SchedulingConfig = {};

  const nodeAffinityRequired = scheduling.nodeAffinityRequired as Array<{
    key: string;
    operator: string;
    values: string;
  }> | undefined;

  if (nodeAffinityRequired && nodeAffinityRequired.length > 0) {
    result.nodeAffinity = result.nodeAffinity || {};
    result.nodeAffinity.required = {
      nodeSelectorTerms: [{
        matchExpressions: nodeAffinityRequired.map(item => ({
          key: item.key,
          operator: item.operator as 'In' | 'NotIn' | 'Exists' | 'DoesNotExist' | 'Gt' | 'Lt',
          values: parseCommaString(item.values),
        })),
      }],
    };
  }

  const nodeAffinityPreferred = scheduling.nodeAffinityPreferred as Array<{
    weight: number;
    key: string;
    operator: string;
    values: string;
  }> | undefined;

  if (nodeAffinityPreferred && nodeAffinityPreferred.length > 0) {
    result.nodeAffinity = result.nodeAffinity || {};
    result.nodeAffinity.preferred = nodeAffinityPreferred.map(item => ({
      weight: item.weight,
      preference: {
        matchExpressions: [{
          key: item.key,
          operator: item.operator as 'In' | 'NotIn' | 'Exists' | 'DoesNotExist' | 'Gt' | 'Lt',
          values: parseCommaString(item.values),
        }],
      },
    }));
  }

  const podAffinityRequired = scheduling.podAffinityRequired as Array<{
    topologyKey: string;
    labelKey: string;
    operator: string;
    labelValues: string;
  }> | undefined;

  if (podAffinityRequired && podAffinityRequired.length > 0) {
    result.podAffinity = result.podAffinity || {};
    result.podAffinity.required = podAffinityRequired.map(item => ({
      topologyKey: item.topologyKey,
      labelSelector: {
        matchExpressions: [{
          key: item.labelKey,
          operator: item.operator as 'In' | 'NotIn' | 'Exists' | 'DoesNotExist',
          values: parseCommaString(item.labelValues),
        }],
      },
    }));
  }

  const podAffinityPreferred = scheduling.podAffinityPreferred as Array<{
    weight: number;
    topologyKey: string;
    labelKey: string;
    operator: string;
    labelValues: string;
  }> | undefined;

  if (podAffinityPreferred && podAffinityPreferred.length > 0) {
    result.podAffinity = result.podAffinity || {};
    result.podAffinity.preferred = podAffinityPreferred.map(item => ({
      weight: item.weight,
      podAffinityTerm: {
        topologyKey: item.topologyKey,
        labelSelector: {
          matchExpressions: [{
            key: item.labelKey,
            operator: item.operator as 'In' | 'NotIn' | 'Exists' | 'DoesNotExist',
            values: parseCommaString(item.labelValues),
          }],
        },
      },
    }));
  }

  const podAntiAffinityRequired = scheduling.podAntiAffinityRequired as Array<{
    topologyKey: string;
    labelKey: string;
    operator: string;
    labelValues: string;
  }> | undefined;

  if (podAntiAffinityRequired && podAntiAffinityRequired.length > 0) {
    result.podAntiAffinity = result.podAntiAffinity || {};
    result.podAntiAffinity.required = podAntiAffinityRequired.map(item => ({
      topologyKey: item.topologyKey,
      labelSelector: {
        matchExpressions: [{
          key: item.labelKey,
          operator: item.operator as 'In' | 'NotIn' | 'Exists' | 'DoesNotExist',
          values: parseCommaString(item.labelValues),
        }],
      },
    }));
  }

  const podAntiAffinityPreferred = scheduling.podAntiAffinityPreferred as Array<{
    weight: number;
    topologyKey: string;
    labelKey: string;
    operator: string;
    labelValues: string;
  }> | undefined;

  if (podAntiAffinityPreferred && podAntiAffinityPreferred.length > 0) {
    result.podAntiAffinity = result.podAntiAffinity || {};
    result.podAntiAffinity.preferred = podAntiAffinityPreferred.map(item => ({
      weight: item.weight,
      podAffinityTerm: {
        topologyKey: item.topologyKey,
        labelSelector: {
          matchExpressions: [{
            key: item.labelKey,
            operator: item.operator as 'In' | 'NotIn' | 'Exists' | 'DoesNotExist',
            values: parseCommaString(item.labelValues),
          }],
        },
      },
    }));
  }

  return Object.keys(result).length > 0 ? result : undefined;
};
