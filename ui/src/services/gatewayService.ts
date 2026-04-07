import { request } from '../utils/api';
import type {
  GatewayClassItem,
  GatewayItem,
  HTTPRouteItem,
  GRPCRouteItem,
  ReferenceGrantItem,
  TopologyData,
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

  // Gateway CRUD
  createGateway: (clusterId: string, namespace: string, yaml: string) =>
    request.post(`/clusters/${clusterId}/gateways`, { namespace, yaml }),

  updateGateway: (clusterId: string, namespace: string, name: string, yaml: string) =>
    request.put(`/clusters/${clusterId}/gateways/${namespace}/${name}`, { yaml }),

  deleteGateway: (clusterId: string, namespace: string, name: string) =>
    request.delete(`/clusters/${clusterId}/gateways/${namespace}/${name}`),

  // HTTPRoute
  listHTTPRoutes: (clusterId: string, namespace?: string): Promise<ListResponse<HTTPRouteItem>> =>
    request.get(`/clusters/${clusterId}/httproutes`, {
      params: namespace ? { namespace } : undefined,
    }),

  getHTTPRoute: (clusterId: string, namespace: string, name: string): Promise<HTTPRouteItem> =>
    request.get(`/clusters/${clusterId}/httproutes/${namespace}/${name}`),

  getHTTPRouteYAML: (clusterId: string, namespace: string, name: string): Promise<{ yaml: string }> =>
    request.get(`/clusters/${clusterId}/httproutes/${namespace}/${name}/yaml`),

  // HTTPRoute CRUD
  createHTTPRoute: (clusterId: string, namespace: string, yaml: string) =>
    request.post(`/clusters/${clusterId}/httproutes`, { namespace, yaml }),

  updateHTTPRoute: (clusterId: string, namespace: string, name: string, yaml: string) =>
    request.put(`/clusters/${clusterId}/httproutes/${namespace}/${name}`, { yaml }),

  deleteHTTPRoute: (clusterId: string, namespace: string, name: string) =>
    request.delete(`/clusters/${clusterId}/httproutes/${namespace}/${name}`),

  // GRPCRoute
  listGRPCRoutes: (clusterId: string, namespace?: string): Promise<ListResponse<GRPCRouteItem>> =>
    request.get(`/clusters/${clusterId}/grpcroutes`, {
      params: namespace ? { namespace } : undefined,
    }),

  getGRPCRoute: (clusterId: string, namespace: string, name: string): Promise<GRPCRouteItem> =>
    request.get(`/clusters/${clusterId}/grpcroutes/${namespace}/${name}`),

  getGRPCRouteYAML: (clusterId: string, namespace: string, name: string): Promise<{ yaml: string }> =>
    request.get(`/clusters/${clusterId}/grpcroutes/${namespace}/${name}/yaml`),

  createGRPCRoute: (clusterId: string, namespace: string, yaml: string) =>
    request.post(`/clusters/${clusterId}/grpcroutes`, { namespace, yaml }),

  updateGRPCRoute: (clusterId: string, namespace: string, name: string, yaml: string) =>
    request.put(`/clusters/${clusterId}/grpcroutes/${namespace}/${name}`, { yaml }),

  deleteGRPCRoute: (clusterId: string, namespace: string, name: string) =>
    request.delete(`/clusters/${clusterId}/grpcroutes/${namespace}/${name}`),

  // ReferenceGrant
  listReferenceGrants: (clusterId: string, namespace?: string): Promise<ListResponse<ReferenceGrantItem>> =>
    request.get(`/clusters/${clusterId}/referencegrants`, {
      params: namespace ? { namespace } : undefined,
    }),

  getReferenceGrantYAML: (clusterId: string, namespace: string, name: string): Promise<{ yaml: string }> =>
    request.get(`/clusters/${clusterId}/referencegrants/${namespace}/${name}/yaml`),

  createReferenceGrant: (clusterId: string, namespace: string, yaml: string) =>
    request.post(`/clusters/${clusterId}/referencegrants`, { namespace, yaml }),

  deleteReferenceGrant: (clusterId: string, namespace: string, name: string) =>
    request.delete(`/clusters/${clusterId}/referencegrants/${namespace}/${name}`),

  // Topology
  getTopology: (clusterId: string): Promise<TopologyData> =>
    request.get(`/clusters/${clusterId}/gateway/topology`),
};
