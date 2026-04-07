import { request } from '../utils/api';
import type {
  GatewayClassItem,
  GatewayItem,
  HTTPRouteItem,
} from '../pages/network/gatewayTypes';

interface ListResponse<T> {
  items: T[];
  total: number;
}

export const gatewayService = {
  // 偵測 Gateway API 是否可用
  getStatus: (clusterId: string): Promise<{ available: boolean }> =>
    request.get(`/clusters/${clusterId}/gateway/status`),

  // GatewayClass
  listGatewayClasses: (clusterId: string): Promise<ListResponse<GatewayClassItem>> =>
    request.get(`/clusters/${clusterId}/gatewayclasses`),

  getGatewayClass: (clusterId: string, name: string): Promise<GatewayClassItem> =>
    request.get(`/clusters/${clusterId}/gatewayclasses/${name}`),

  // Gateway
  listGateways: (clusterId: string, namespace?: string): Promise<ListResponse<GatewayItem>> =>
    request.get(`/clusters/${clusterId}/gateways`, {
      params: namespace ? { namespace } : undefined,
    }),

  getGateway: (clusterId: string, namespace: string, name: string): Promise<GatewayItem> =>
    request.get(`/clusters/${clusterId}/gateways/${namespace}/${name}`),

  getGatewayYAML: (clusterId: string, namespace: string, name: string): Promise<{ yaml: string }> =>
    request.get(`/clusters/${clusterId}/gateways/${namespace}/${name}/yaml`),

  // HTTPRoute
  listHTTPRoutes: (clusterId: string, namespace?: string): Promise<ListResponse<HTTPRouteItem>> =>
    request.get(`/clusters/${clusterId}/httproutes`, {
      params: namespace ? { namespace } : undefined,
    }),

  getHTTPRoute: (clusterId: string, namespace: string, name: string): Promise<HTTPRouteItem> =>
    request.get(`/clusters/${clusterId}/httproutes/${namespace}/${name}`),

  getHTTPRouteYAML: (clusterId: string, namespace: string, name: string): Promise<{ yaml: string }> =>
    request.get(`/clusters/${clusterId}/httproutes/${namespace}/${name}/yaml`),
};
