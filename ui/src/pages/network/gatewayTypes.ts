export interface GatewayTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

export interface GatewayK8sCondition {
  type: string;
  status: 'True' | 'False' | 'Unknown';
  reason: string;
  message: string;
}

export interface GatewayClassItem {
  name: string;
  controller: string;
  description: string;
  acceptedStatus: string; // "Accepted" | "Unknown" | reason string
  createdAt: string;
}

export interface GatewayListener {
  name: string;
  port: number;
  protocol: 'HTTP' | 'HTTPS' | 'TLS' | 'TCP' | 'UDP' | string;
  hostname: string;
  tlsMode: string;
  status: string; // "Ready" | "Unknown" | reason
}

export interface GatewayAddress {
  type: 'IPAddress' | 'Hostname' | string;
  value: string;
}

export interface GatewayItem {
  name: string;
  namespace: string;
  gatewayClass: string;
  listeners: GatewayListener[];
  addresses: GatewayAddress[];
  conditions: GatewayK8sCondition[];
  createdAt: string;
}

export interface HTTPRouteBackend {
  name: string;
  namespace: string;
  port: number;
  weight: number;
}

export interface HTTPRouteRule {
  matches: Record<string, unknown>[];
  filters: Record<string, unknown>[];
  backends: HTTPRouteBackend[];
}

export interface HTTPRouteParentRef {
  gatewayNamespace: string;
  gatewayName: string;
  sectionName: string;
  conditions: GatewayK8sCondition[];
}

// --- Form value types（Phase 2）---

export interface GatewayListenerFormValue {
  name: string;
  port: number;
  protocol: string;
  hostname?: string;
  tlsMode?: string;
  tlsSecretName?: string;
  tlsSecretNamespace?: string;
}

export interface GatewayFormValues {
  name: string;
  namespace: string;
  gatewayClass: string;
  listeners: GatewayListenerFormValue[];
}

export interface HTTPRouteMatchFormValue {
  pathType: 'Exact' | 'PathPrefix' | 'RegularExpression';
  pathValue: string;
}

export interface HTTPRouteBackendFormValue {
  name: string;
  port: number;
  weight: number;
}

export interface HTTPRouteRuleFormValue {
  matches: HTTPRouteMatchFormValue[];
  backends: HTTPRouteBackendFormValue[];
}

export interface HTTPRouteParentRefFormValue {
  gatewayName: string;
  gatewayNamespace: string;
  sectionName?: string;
}

export interface HTTPRouteFormValues {
  name: string;
  namespace: string;
  hostnames: string[];
  parentRefs: HTTPRouteParentRefFormValue[];
  rules: HTTPRouteRuleFormValue[];
}

export interface HTTPRouteItem {
  name: string;
  namespace: string;
  hostnames: string[];
  parentRefs: HTTPRouteParentRef[];
  rules: HTTPRouteRule[];
  conditions: GatewayK8sCondition[];
  createdAt: string;
}
