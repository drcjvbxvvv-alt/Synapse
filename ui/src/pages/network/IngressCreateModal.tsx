import React, { useState, useEffect, useCallback } from 'react';
import { Modal, Tabs, Form, Button, App, Alert } from 'antd';
import { CheckCircleOutlined, ExclamationCircleOutlined } from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import IngressFormContent from './IngressFormContent';
import * as YAML from 'yaml';
import { IngressService } from '../../services/ingressService';
import { ResourceService } from '../../services/resourceService';
import { ServiceService } from '../../services/serviceService';
import { getNamespaces } from '../../services/configService';
import { useTranslation } from 'react-i18next';
import { parseApiError } from '../../utils/api';
import { prefetchMonaco } from '../../utils/prefetch';
import type { Service } from '../../types';

interface KubernetesIngressYAML {
  apiVersion: string;
  kind: string;
  metadata: {
    name: string;
    namespace: string;
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  spec: {
    ingressClassName?: string;
    rules?: Array<{
      host: string;
      http: {
        paths: Array<{
          path: string;
          pathType: string;
          backend: {
            service: {
              name: string;
              port: {
                number: number;
              };
            };
          };
        }>;
      };
    }>;
    tls?: Array<{
      hosts: string[];
      secretName: string;
    }>;
  };
}

interface IngressCreateModalProps {
  visible: boolean;
  clusterId: string;
  onClose: () => void;
  onSuccess: () => void;
}

interface RuleFormItem {
  host: string;
  paths?: Array<{
    path: string;
    pathType: string;
    serviceName: string;
    servicePort: number | string;
  }>;
}

interface TLSFormItem {
  hosts: string;
  secretName: string;
}

interface LabelFormItem {
  key: string;
  value: string;
}

const IngressCreateModal: React.FC<IngressCreateModalProps> = ({
  visible,
  clusterId,
  onClose,
  onSuccess,
}) => {
  const { message } = App.useApp();
const { t } = useTranslation(['network', 'common']);
const [form] = Form.useForm();
  const [activeTab, setActiveTab] = useState('form');
  const [yamlContent, setYamlContent] = useState(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  namespace: default
spec:
  ingressClassName: nginx
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 80`);
  const [loading, setLoading] = useState(false);
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<{ success: boolean; message: string } | null>(null);

  // 命名空間列表
  const [namespaces, setNamespaces] = useState<string[]>(['default']);
  const [loadingNamespaces, setLoadingNamespaces] = useState(false);

  // Service 列表（依命名空間）
  const [services, setServices] = useState<Service[]>([]);
  const [loadingServices, setLoadingServices] = useState(false);

  const loadServices = useCallback(async (ns: string) => {
    if (!clusterId || !ns) return;
    setLoadingServices(true);
    try {
      const resp = await ServiceService.getServices(clusterId, ns, undefined, undefined, 1, 200);
      const items = (resp as unknown as { items: Service[] }).items ?? [];
      setServices(items);
    } catch {
      setServices([]);
    } finally {
      setLoadingServices(false);
    }
  }, [clusterId]);

  // 載入命名空間列表
  useEffect(() => {
    const loadNamespaces = async () => {
      if (!clusterId || !visible) return;
      setLoadingNamespaces(true);
      try {
        const nsList = await getNamespaces(Number(clusterId));
        setNamespaces(nsList);
        // 預設載入 default namespace 的 services
        await loadServices('default');
      } catch (error) {
        console.error('載入命名空間失敗:', error);
      } finally {
        setLoadingNamespaces(false);
      }
    };

    loadNamespaces();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clusterId, visible]);

  // 修補 YAML 中 Ingress path 缺少 pathType 的問題
  const patchIngressPathType = (raw: string): string => {
    try {
      const doc = YAML.parse(raw) as Record<string, unknown>;
      if ((doc?.kind as string) !== 'Ingress') return raw;
      const rules = ((doc?.spec as Record<string, unknown>)?.rules as Record<string, unknown>[]) || [];
      rules.forEach((rule) => {
        const paths = ((rule?.http as Record<string, unknown>)?.paths as Record<string, unknown>[]) || [];
        paths.forEach((p) => {
          if (!p.pathType) p.pathType = 'Prefix';
          if (!p.path) p.path = '/';
        });
      });
      return YAML.stringify(doc);
    } catch {
      return raw;
    }
  };

  const handleSubmit = async () => {
    setLoading(true);
    try {
      if (activeTab === 'yaml') {
        // YAML方式建立：補齊缺少的 pathType
        const patchedYaml = patchIngressPathType(yamlContent);
        await IngressService.createIngress(clusterId, {
          namespace: 'default',
          yaml: patchedYaml,
        });
        
        message.success(t('network:create.ingressSuccess'));
        onSuccess();
        onClose();
      } else {
        const values = await form.validateFields();
        
        await IngressService.createIngress(clusterId, {
          namespace: values.namespace,
          formData: {
            name: values.name,
            ingressClassName: values.ingressClassName || null,
            rules: (values.rules as RuleFormItem[] | undefined)?.map((rule) => ({
              host: rule.host,
              paths: rule.paths?.map((path) => ({
                path: path.path,
                pathType: path.pathType,
                serviceName: path.serviceName,
                servicePort: path.servicePort,
              })) || [],
            })) || [],
            tls: (values.tls as TLSFormItem[] | undefined)?.map((t) => ({
              hosts: t.hosts?.split(',').map((h: string) => h.trim()) || [],
              secretName: t.secretName,
            })) || [],
            labels: (values.labels as LabelFormItem[] | undefined)?.reduce((acc: Record<string, string>, item) => {
              acc[item.key] = item.value;
              return acc;
            }, {}) || {},
            annotations: (values.annotations as LabelFormItem[] | undefined)?.reduce((acc: Record<string, string>, item) => {
              acc[item.key] = item.value;
              return acc;
            }, {}) || {},
          },
        });
        
        message.success(t('network:create.ingressSuccess'));
        form.resetFields();
        onSuccess();
        onClose();
      }
    } catch (error: unknown) {
      console.error('Failed to create Ingress:', error);
      const err = error as { message?: string };
      message.error(err.message || t('network:create.ingressFailed'));
    } finally {
      setLoading(false);
    }
  };

  // 預檢（Dry Run）
  const handleDryRun = async () => {
    let currentYaml = yamlContent;
    if (activeTab === 'form') {
      formToYaml();
      // formToYaml sets state async; build inline for immediate use
      try {
        const values = form.getFieldsValue();
        const obj = {
          apiVersion: 'networking.k8s.io/v1',
          kind: 'Ingress',
          metadata: { name: values.name || 'my-ingress', namespace: values.namespace || 'default' },
          spec: {
            ...(values.ingressClassName ? { ingressClassName: values.ingressClassName } : {}),
            rules: ((values.rules as RuleFormItem[] | undefined) || []).map((rule) => ({
              host: rule.host,
              http: {
                paths: (rule.paths || []).map((p) => ({
                  path: p.path,
                  pathType: p.pathType,
                  backend: { service: { name: p.serviceName, port: { number: typeof p.servicePort === 'string' ? parseInt(p.servicePort, 10) : p.servicePort } } },
                })),
              },
            })),
          },
        };
        currentYaml = YAML.stringify(obj);
      } catch {
        message.error(t('network:create.yamlParseError'));
        return;
      }
    }

    try {
      YAML.parse(currentYaml);
    } catch (err) {
      message.error(t('network:editPage.yamlFormatError', { error: err instanceof Error ? err.message : String(err) }));
      return;
    }

    // 補齊缺少的 pathType，避免預檢誤報
    currentYaml = patchIngressPathType(currentYaml);

    setDryRunning(true);
    setDryRunResult(null);
    try {
      await ResourceService.applyYAML(clusterId, 'Ingress', currentYaml, true);
      setDryRunResult({ success: true, message: t('network:editPage.dryRunSuccess') });
    } catch (err) {
      setDryRunResult({ success: false, message: parseApiError(err) || t('network:editPage.dryRunFailed') });
    } finally {
      setDryRunning(false);
    }
  };

  const handleCancel = () => {
    form.resetFields();
    setDryRunResult(null);
    setYamlContent(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  namespace: default
spec:
  ingressClassName: nginx
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 80`);
    onClose();
  };

  // 表單轉YAML
  const formToYaml = () => {
    try {
      const values = form.getFieldsValue();
      
      const ingressObj: KubernetesIngressYAML = {
        apiVersion: 'networking.k8s.io/v1',
        kind: 'Ingress',
        metadata: {
          name: values.name || 'my-ingress',
          namespace: values.namespace || 'default',
        },
        spec: {
          ingressClassName: values.ingressClassName || 'nginx',
          rules: (values.rules as RuleFormItem[] | undefined)?.map((rule) => ({
            host: rule.host,
            http: {
              paths: rule.paths?.map((path) => ({
                path: path.path || '/',
                pathType: path.pathType || 'Prefix',
                backend: {
                  service: {
                    name: path.serviceName,
                    port: {
                      number: typeof path.servicePort === 'string' ? parseInt(path.servicePort, 10) : path.servicePort,
                    },
                  },
                },
              })) || [],
            },
          })).filter((r) => r.host) || [
            {
              host: 'example.com',
              http: {
                paths: [
                  {
                    path: '/',
                    pathType: 'Prefix',
                    backend: {
                      service: {
                        name: 'my-service',
                        port: { number: 80 },
                      },
                    },
                  },
                ],
              },
            },
          ],
        },
      };

      // {t('network:create.addTLS')}配置（如果存在）
      if (values.tls && Array.isArray(values.tls) && values.tls.length > 0) {
        ingressObj.spec.tls = (values.tls as TLSFormItem[])
          .map((t) => ({
            hosts: t.hosts?.split(',').map((h: string) => h.trim()).filter((h: string) => h) || [],
            secretName: t.secretName,
          }))
          .filter((t) => t.hosts.length > 0 && t.secretName);
      }

      // 新增labels和annotations（如果存在）
      if (values.labels && Array.isArray(values.labels) && values.labels.length > 0) {
        ingressObj.metadata.labels = (values.labels as LabelFormItem[]).reduce((acc: Record<string, string>, item) => {
          if (item.key) acc[item.key] = item.value;
          return acc;
        }, {});
      }

      if (values.annotations && Array.isArray(values.annotations) && values.annotations.length > 0) {
        ingressObj.metadata.annotations = (values.annotations as LabelFormItem[]).reduce((acc: Record<string, string>, item) => {
          if (item.key) acc[item.key] = item.value;
          return acc;
        }, {});
      }

      const yaml = YAML.stringify(ingressObj);
      setYamlContent(yaml);
    } catch (error) {
      console.error('表單轉YAML失敗:', error);
    }
  };

  // YAML轉表單
  const yamlToForm = () => {
    try {
      const ingressObj = YAML.parse(yamlContent);
      
      // 提取rules
      interface ParsedRule {
        host?: string;
        http?: {
          paths?: Array<{
            path?: string;
            pathType?: string;
            backend?: {
              service?: {
                name?: string;
                port?: {
                  number?: number | string;
                };
              };
            };
          }>;
        };
      }

      interface ParsedTLS {
        hosts?: string[];
        secretName?: string;
      }

      const rules = ((ingressObj.spec?.rules as ParsedRule[] | undefined) || []).map((rule) => ({
        host: rule.host || '',
        paths: (rule.http?.paths || []).map((path) => ({
          path: path.path || '/',
          pathType: path.pathType || 'Prefix',
          serviceName: path.backend?.service?.name || '',
          servicePort: path.backend?.service?.port?.number || 80,
        })),
      }));

      // 提取TLS
      const tls = ((ingressObj.spec?.tls as ParsedTLS[] | undefined) || []).map((t) => ({
        hosts: t.hosts?.join(', ') || '',
        secretName: t.secretName || '',
      }));

      // 提取labels
      const labels = ingressObj.metadata?.labels
        ? Object.entries(ingressObj.metadata.labels).map(([key, value]) => ({ key, value }))
        : [];

      // 提取annotations
      const annotations = ingressObj.metadata?.annotations
        ? Object.entries(ingressObj.metadata.annotations).map(([key, value]) => ({ key, value }))
        : [];

      form.setFieldsValue({
        namespace: ingressObj.metadata?.namespace || 'default',
        name: ingressObj.metadata?.name || '',
        ingressClassName: ingressObj.spec?.ingressClassName || 'nginx',
        rules: rules.length > 0 ? rules : [{ paths: [{ pathType: 'Prefix' }] }],
        tls: tls.length > 0 ? tls : undefined,
        labels: labels.length > 0 ? labels : undefined,
        annotations: annotations.length > 0 ? annotations : undefined,
      });
    } catch (error) {
      console.error('YAML轉表單失敗:', error);
      message.error(t('network:create.yamlParseError'));
    }
  };

  // 處理Tab切換
  const handleTabChange = (key: string) => {
    if (key === 'yaml' && activeTab === 'form') {
      // 表單 -> YAML
      formToYaml();
    } else if (key === 'form' && activeTab === 'yaml') {
      // YAML -> 表單
      yamlToForm();
    }
    setActiveTab(key);
  };

  const formContent = (
    <IngressFormContent
      form={form}
      namespaces={namespaces}
      loadingNamespaces={loadingNamespaces}
      services={services}
      loadingServices={loadingServices}
      onNamespaceChange={loadServices}
    />
  );

  const yamlEditor = (
    <MonacoEditor
      height="500px"
      language="yaml"
      value={yamlContent}
      onChange={(value) => setYamlContent(value || '')}
      options={{
        minimap: { enabled: false },
        fontSize: 14,
        wordWrap: 'on',
        scrollBeyondLastLine: false,
      }}
    />
  );

  return (
    <Modal
      title={t('network:create.ingressTitle')}
      open={visible}
      onCancel={handleCancel}
      width={900}
      destroyOnHidden
      footer={[
        <Button key="cancel" onClick={handleCancel}>
          {t('common:actions.cancel')}
        </Button>,
        <Button
          key="dryrun"
          icon={<CheckCircleOutlined />}
          loading={dryRunning}
          onClick={handleDryRun}
        >
          {t('network:editPage.dryRun')}
        </Button>,
        <Button key="submit" type="primary" loading={loading} onClick={handleSubmit}>
          {t('common:actions.create')}
        </Button>,
      ]}
    >
      {dryRunResult && (
        <Alert
          message={dryRunResult.success ? t('network:editPage.dryRunSuccessTitle') : t('network:editPage.dryRunFailedTitle')}
          description={dryRunResult.message}
          type={dryRunResult.success ? 'success' : 'error'}
          showIcon
          icon={dryRunResult.success ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />}
          closable
          onClose={() => setDryRunResult(null)}
          style={{ marginBottom: 12 }}
        />
      )}
      <Tabs
        activeKey={activeTab}
        onChange={handleTabChange}
        items={[
          {
            key: 'form',
            label: t('network:create.formMode'),
            children: <div style={{ maxHeight: 600, overflowY: 'auto' }}>{formContent}</div>,
          },
          {
            key: 'yaml',
            label: <span onMouseEnter={prefetchMonaco}>{t('network:create.yamlMode')}</span>,
            children: yamlEditor,
          },
        ]}
      />
    </Modal>
  );
};

export default IngressCreateModal;

