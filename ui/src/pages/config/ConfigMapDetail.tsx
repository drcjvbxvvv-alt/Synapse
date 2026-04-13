import React, { useCallback, useEffect, useState } from 'react';
import {
  Card,
  Descriptions,
  Space,
  Button,
  Tag,
  message,
  Tabs,
  Typography,
  Table,
  Popconfirm,
  App,
} from 'antd';
import {
  ArrowLeftOutlined,
  EditOutlined,
  DeleteOutlined,
  ReloadOutlined,
  HistoryOutlined,
  RollbackOutlined,
} from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import { configMapService, type ConfigMapDetail as ConfigMapDetailType, type ConfigVersion } from '../../services/configService';
import { usePermission } from '../../hooks/usePermission';
import MonacoEditor from '@monaco-editor/react';
import { useTranslation } from 'react-i18next';
import { showApiError } from '../../utils/api';
import PageSkeleton from '../../components/PageSkeleton';

const { Title, Text } = Typography;
const { TabPane } = Tabs;

const ConfigMapDetail: React.FC = () => {
  const navigate = useNavigate();
  const { clusterId, namespace, name } = useParams<{
    clusterId: string;
    namespace: string;
    name: string;
  }>();
  const { t } = useTranslation(['config', 'common']);
  const { modal } = App.useApp();
  const { hasFeature } = usePermission();
  const [loading, setLoading] = useState(false);
  const [configMap, setConfigMap] = useState<ConfigMapDetailType | null>(null);
  const [versions, setVersions] = useState<ConfigVersion[]>([]);
  const [versionsLoading, setVersionsLoading] = useState(false);

  // 載入ConfigMap詳情
  const loadConfigMap = React.useCallback(async () => {
    if (!clusterId || !namespace || !name) return;
    setLoading(true);
    try {
      const data = await configMapService.getConfigMap(
        Number(clusterId),
        namespace,
        name
      );
      setConfigMap(data);
    } catch (error: unknown) {
      showApiError(error, t('config:detail.loadConfigMapError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, name, t]);

  const loadVersions = useCallback(async () => {
    if (!clusterId || !namespace || !name) return;
    setVersionsLoading(true);
    try {
      const data = await configMapService.getVersions(Number(clusterId), namespace, name);
      setVersions(data || []);
    } catch {
      // versions may not exist yet, silently ignore
    } finally {
      setVersionsLoading(false);
    }
  }, [clusterId, namespace, name]);

  useEffect(() => {
    loadConfigMap();
    loadVersions();
  }, [loadConfigMap, loadVersions]);

  const handleRollback = async (version: number) => {
    if (!clusterId || !namespace || !name) return;
    try {
      await configMapService.rollback(Number(clusterId), namespace, name, version);
      message.success(`已回滾至版本 v${version}`);
      loadConfigMap();
      loadVersions();
    } catch (error: unknown) {
      showApiError(error, '回滾失敗');
    }
  };

  // 刪除ConfigMap
  const handleDelete = () => {
    modal.confirm({
      title: t('common:messages.confirmDelete'),
      content: t('config:detail.confirmDeleteConfigMap', { name }),
      onOk: async () => {
        if (!clusterId || !namespace || !name) return;
        try {
            await configMapService.deleteConfigMap(Number(clusterId), namespace, name);
          message.success(t('config:detail.deleteConfigMapSuccess'));
          navigate(`/clusters/${clusterId}/configs`);
        } catch (error: unknown) {
          showApiError(error, t('config:detail.deleteConfigMapError'));
        }
      },
    });
  };

  if (loading) return <PageSkeleton variant="detail" />;

  if (!configMap) {
    return (
      <Card>
        <div style={{ textAlign: 'center', padding: '50px' }}>
          <Text>{t('config:detail.configMapNotExist')}</Text>
        </div>
      </Card>
    );
  }

  return (
    <div style={{ padding: '24px' }}>
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        {/* 頭部操作欄 */}
        <Card>
          <Space style={{ width: '100%', justifyContent: 'space-between' }}>
            <Space>
              <Button
                icon={<ArrowLeftOutlined />}
                onClick={() => navigate(`/clusters/${clusterId}/configs`)}
              >
                {t('common:actions.back')}
              </Button>
              <Title level={4} style={{ margin: 0 }}>
                ConfigMap: {configMap.name}
              </Title>
            </Space>
            <Space>
              <Button icon={<ReloadOutlined />} onClick={loadConfigMap}>
                {t('common:actions.refresh')}
              </Button>
              <Button
                icon={<EditOutlined />}
                onClick={() =>
                  navigate(`/clusters/${clusterId}/configs/configmap/${namespace}/${name}/edit`)
                }
              >
                {t('common:actions.edit')}
              </Button>
              {hasFeature('config:delete') && (
                <Button icon={<DeleteOutlined />} danger onClick={handleDelete}>
                  {t('common:actions.delete')}
                </Button>
              )}
            </Space>
          </Space>
        </Card>

        {/* 基本資訊 */}
        <Card title={t('config:detail.basicInfo')}>
          <Descriptions bordered column={2}>
            <Descriptions.Item label={t('config:detail.name')}>{configMap.name}</Descriptions.Item>
            <Descriptions.Item label={t('config:detail.namespace')}>
              <Tag color="blue">{configMap.namespace}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t('config:detail.createdAt')}>
              {new Date(configMap.creationTimestamp).toLocaleString('zh-TW')}
            </Descriptions.Item>
            <Descriptions.Item label={t('config:detail.age')}>
              {configMap.age}
            </Descriptions.Item>
            <Descriptions.Item label={t('config:detail.resourceVersion')}>
              {configMap.resourceVersion}
            </Descriptions.Item>
          </Descriptions>
        </Card>

        {/* 標籤和註解 */}
        <Card title={t('config:detail.labelsAndAnnotations')}>
          <Tabs defaultActiveKey="labels">
            <TabPane tab={t('config:detail.labels')} key="labels">
              <Space size={[0, 8]} wrap>
                {Object.entries(configMap.labels || {}).length > 0 ? (
                  Object.entries(configMap.labels).map(([key, value]) => (
                    <Tag key={key} color="blue">
                      {key}: {value}
                    </Tag>
                  ))
                ) : (
                  <Text type="secondary">{t('config:detail.noLabels')}</Text>
                )}
              </Space>
            </TabPane>
            <TabPane tab={t('config:detail.annotations')} key="annotations">
              <Space size={[0, 8]} wrap direction="vertical" style={{ width: '100%' }}>
                {Object.entries(configMap.annotations || {}).length > 0 ? (
                  Object.entries(configMap.annotations).map(([key, value]) => (
                    <div key={key}>
                      <Text strong>{key}:</Text> <Text>{value}</Text>
                    </div>
                  ))
                ) : (
                  <Text type="secondary">{t('config:detail.noAnnotations')}</Text>
                )}
              </Space>
            </TabPane>
          </Tabs>
        </Card>

        {/* 資料內容 */}
        <Card title={t('config:detail.dataContent')}>
          {Object.entries(configMap.data || {}).length > 0 ? (
            <Tabs type="card">
              {Object.entries(configMap.data).map(([key, value]) => (
                <TabPane tab={key} key={key}>
                  <div style={{ border: '1px solid #d9d9d9', borderRadius: '4px' }}>
                    <MonacoEditor
                      height="400px"
                      language="plaintext"
                      value={value}
                      options={{
                        readOnly: true,
                        minimap: { enabled: false },
                        lineNumbers: 'on',
                        scrollBeyondLastLine: false,
                        automaticLayout: true,
                      }}
                      theme="vs-light"
                    />
                  </div>
                </TabPane>
              ))}
            </Tabs>
          ) : (
            <Text type="secondary">{t('config:detail.noData')}</Text>
          )}
        </Card>

        {/* 版本歷史 */}
        <Card
          title={<Space><HistoryOutlined />版本歷史</Space>}
          extra={<Button size="small" icon={<ReloadOutlined />} onClick={loadVersions}>重新整理</Button>}
        >
          <Table<ConfigVersion>
            scroll={{ x: 'max-content' }}
            loading={versionsLoading}
            dataSource={versions}
            rowKey="id"
            size="small"
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: '暫無版本記錄（首次編輯後開始追蹤）' }}
            columns={[
              { title: '版本', dataIndex: 'version', width: 70, render: (v: number) => `v${v}` },
              { title: '操作人', dataIndex: 'changedBy', width: 120 },
              {
                title: '時間',
                dataIndex: 'changedAt',
                render: (v: string) => new Date(v).toLocaleString('zh-TW'),
              },
              {
                title: '操作',
                width: 100,
                render: (_: unknown, record: ConfigVersion) => (
                  <Popconfirm
                    title={`確定回滾至 v${record.version}？`}
                    onConfirm={() => handleRollback(record.version)}
                    okText="確定"
                    cancelText="取消"
                  >
                    <Button size="small" icon={<RollbackOutlined />} type="link">回滾</Button>
                  </Popconfirm>
                ),
              },
            ]}
          />
        </Card>
      </Space>
    </div>
  );
};

export default ConfigMapDetail;

