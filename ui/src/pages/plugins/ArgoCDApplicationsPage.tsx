import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Badge,
  Form,
  Popconfirm,
  Statistic,
  Row,
  Col,
  Empty,
  message,
  Tooltip,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  PlusOutlined,
  SyncOutlined,
  ReloadOutlined,
  BranchesOutlined,
  DeleteOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  ClockCircleOutlined,
  SettingOutlined,
  EyeOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
  QuestionCircleOutlined,
} from '@ant-design/icons';
import { argoCDService } from '../../services/argoCDService';
import { useTranslation } from 'react-i18next';
import { usePermission } from '../../hooks/usePermission';
import type {
  ArgoCDApplication,
  CreateApplicationRequest,
} from '../../services/argoCDService';
import ArgoCDAppFormModal from './ArgoCDAppFormModal';
import ArgoCDAppDetailDrawer from './ArgoCDAppDetailDrawer';

const ArgoCDApplicationsPage: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const navigate = useNavigate();
  
const { t } = useTranslation(['plugins', 'common']);
const { canWrite } = usePermission();
const [applications, setApplications] = useState<ArgoCDApplication[]>([]);
  const [loading, setLoading] = useState(false);
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [detailDrawerVisible, setDetailDrawerVisible] = useState(false);
  const [selectedApp, setSelectedApp] = useState<ArgoCDApplication | null>(null);
  const [form] = Form.useForm();
  const [creating, setCreating] = useState(false);
  const [configEnabled, setConfigEnabled] = useState(false);
  const [configLoading, setConfigLoading] = useState(true);

  // 載入配置狀態
  const loadConfig = useCallback(async () => {
    if (!clusterId) return;
    try {
      setConfigLoading(true);
      const response = await argoCDService.getConfig(clusterId);
      setConfigEnabled(response?.enabled || false);
    } catch (error) {
      console.error('載入配置失敗:', error);
      setConfigEnabled(false);
    } finally {
      setConfigLoading(false);
    }
  }, [clusterId]);

  // 載入應用列表
  const loadApplications = useCallback(async () => {
    if (!clusterId || !configEnabled) return;
    setLoading(true);
    try {
      const response = await argoCDService.listApplications(clusterId);
      setApplications(response.items || []);
    } catch (error: unknown) {
      console.error('載入應用列表失敗:', error);
    } finally {
      setLoading(false);
    }
  }, [clusterId, configEnabled]);

  // 先載入配置狀態
  useEffect(() => {
    loadConfig();
  }, [loadConfig]);

  // 配置啟用後載入應用列表
  useEffect(() => {
    if (configEnabled) {
    loadApplications();
    }
  }, [configEnabled, loadApplications]);

  // 建立應用
  const handleCreate = async () => {
    try {
      const values = await form.validateFields();
      setCreating(true);
      
      const req: CreateApplicationRequest = {
        name: values.name,
        namespace: 'argocd',
        path: values.path,
        target_revision: values.target_revision || 'HEAD',
        dest_namespace: values.dest_namespace,
        auto_sync: values.auto_sync || false,
        self_heal: values.self_heal || false,
        prune: values.prune || false,
        helm_values: values.helm_values,
      };
      
      await argoCDService.createApplication(clusterId!, req);
      message.success(t('plugins:argocd.createSuccess'));
      setCreateModalVisible(false);
      form.resetFields();
      loadApplications();
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : t('plugins:argocd.createFailed');
      message.error(errorMessage);
    } finally {
      setCreating(false);
    }
  };

  // 同步應用
  const handleSync = async (appName: string) => {
    try {
      message.loading({ content: t('plugins:argocd.syncing'), key: 'sync' });
      await argoCDService.syncApplication(clusterId!, appName);
      message.success({ content: t('plugins:argocd.syncTriggered'), key: 'sync' });
      loadApplications();
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : t('plugins:argocd.syncFailed');
      message.error({ content: errorMessage, key: 'sync' });
    }
  };

  // 刪除應用
  const handleDelete = async (appName: string) => {
    try {
      message.loading({ content: t('plugins:argocd.deleting'), key: 'delete' });
      await argoCDService.deleteApplication(clusterId!, appName, true);
      message.success({ content: t('plugins:argocd.deleteSuccess'), key: 'delete' });
      loadApplications();
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : t('plugins:argocd.deleteFailed');
      message.error({ content: errorMessage, key: 'delete' });
    }
  };

  // 檢視詳情
  const handleViewDetail = (app: ArgoCDApplication) => {
    setSelectedApp(app);
    setDetailDrawerVisible(true);
  };

  // 回滾應用
  const handleRollback = async (appName: string, revisionId: number) => {
    try {
      message.loading({ content: t('plugins:argocd.rolling'), key: 'rollback' });
      await argoCDService.rollbackApplication(clusterId!, appName, { revision_id: revisionId });
      message.success({ content: t('plugins:argocd.rollbackSuccess'), key: 'rollback' });
      loadApplications();
      setDetailDrawerVisible(false);
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : t('plugins:argocd.rollbackFailed');
      message.error({ content: errorMessage, key: 'rollback' });
    }
  };

  // 同步狀態標籤
  const getSyncStatusTag = (status: string) => {
    const config: Record<string, { color: string; icon: React.ReactNode }> = {
      'Synced': { color: 'success', icon: <CheckCircleOutlined /> },
      'OutOfSync': { color: 'warning', icon: <ExclamationCircleOutlined /> },
      'Unknown': { color: 'default', icon: <QuestionCircleOutlined /> },
    };
    const cfg = config[status] || config['Unknown'];
    return <Tag color={cfg.color} icon={cfg.icon}>{status || 'Unknown'}</Tag>;
  };

  // 健康狀態標籤
  const getHealthStatusBadge = (status: string) => {
    const config: Record<string, { status: 'success' | 'error' | 'processing' | 'warning' | 'default'; icon?: React.ReactNode }> = {
      'Healthy': { status: 'success', icon: <CheckCircleOutlined /> },
      'Degraded': { status: 'error', icon: <CloseCircleOutlined /> },
      'Progressing': { status: 'processing', icon: <LoadingOutlined /> },
      'Suspended': { status: 'warning', icon: <ClockCircleOutlined /> },
      'Missing': { status: 'default' },
      'Unknown': { status: 'default' },
    };
    const cfg = config[status] || config['Unknown'];
    return <Badge status={cfg.status} text={status || 'Unknown'} />;
  };

  // 統計資料
  const stats = {
    total: applications.length,
    synced: applications.filter(a => a.sync_status === 'Synced').length,
    outOfSync: applications.filter(a => a.sync_status === 'OutOfSync').length,
    healthy: applications.filter(a => a.health_status === 'Healthy').length,
    degraded: applications.filter(a => a.health_status === 'Degraded').length,
  };

  // 表格列定義
  const columns: ColumnsType<ArgoCDApplication> = [
    {
      title: t('plugins:argocd.appName'),
      dataIndex: 'name',
      key: 'name',
      fixed: 'left',
      width: 200,
      render: (text: string, record: ArgoCDApplication) => (
        <Button type="link" onClick={() => handleViewDetail(record)} style={{ padding: 0 }}>
          {text}
        </Button>
      ),
    },
    {
      title: t('plugins:argocd.project'),
      dataIndex: 'project',
      key: 'project',
      width: 120,
      render: (text: string) => <Tag>{text || 'default'}</Tag>,
    },
    {
      title: t('plugins:argocd.syncStatus'),
      dataIndex: 'sync_status',
      key: 'sync_status',
      width: 120,
      render: getSyncStatusTag,
    },
    {
      title: t('plugins:argocd.healthStatus'),
      dataIndex: 'health_status',
      key: 'health_status',
      width: 120,
      render: getHealthStatusBadge,
    },
    {
      title: t('plugins:argocd.gitPath'),
      dataIndex: ['source', 'path'],
      key: 'path',
      width: 200,
      ellipsis: true,
      render: (text: string, record: ArgoCDApplication) => (
        <Tooltip title={record.source?.repo_url}>
          <Space>
            <BranchesOutlined />
            <span>{text || '-'}</span>
          </Space>
        </Tooltip>
      ),
    },
    {
      title: t('common:table.version'),
      dataIndex: 'synced_revision',
      key: 'revision',
      width: 100,
      render: (text: string) => (
        <Tooltip title={text}>
          <code>{text?.substring(0, 7) || '-'}</code>
        </Tooltip>
      ),
    },
    {
      title: t('plugins:argocd.destNamespace'),
      dataIndex: ['destination', 'namespace'],
      key: 'dest_namespace',
      width: 130,
      render: (text: string) => <Tag color="blue">{text || '-'}</Tag>,
    },
    ...(canWrite() ? [{
      title: t('common:table.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 200,
      render: (_: unknown, record: ArgoCDApplication) => (
        <Space size="small">
          <Tooltip title={t('plugins:argocd.sync')}>
            <Button
              type="primary"
              size="small"
              icon={<SyncOutlined />}
              onClick={() => handleSync(record.name)}
            />
          </Tooltip>
          <Tooltip title={t('plugins:argocd.appDetail')}>
            <Button
              size="small"
              icon={<EyeOutlined />}
              onClick={() => handleViewDetail(record)}
            />
          </Tooltip>
          <Popconfirm
            title={t('plugins:argocd.deleteConfirm')}
            description={t('plugins:argocd.deleteConfirmDesc')}
            onConfirm={() => handleDelete(record.name)}
            okText={t('common:actions.confirm')}
            cancelText={t('common:actions.cancel')}
          >
            <Tooltip title={t('common:actions.delete')}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    }] : []),
  ];

  // 載入配置狀態中
  if (configLoading) {
    return (
      <div style={{ 
        display: 'flex', 
        justifyContent: 'center', 
        alignItems: 'center', 
        height: '60vh' 
      }}>
        <LoadingOutlined style={{ fontSize: 32 }} spin />
      </div>
    );
  }

  // 如果未啟用配置，顯示提示（類似告警中心的設計）
  if (!configEnabled) {
    return (
      <div style={{ padding: 24 }}>
        <Card>
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={
              <Space direction="vertical">
                <span>{t('plugins:argocd.notConfigured')}</span>
                <span style={{ color: '#999' }}>{t('plugins:argocd.notConfiguredDesc')}</span>
              </Space>
            }
          >
            <Space>
              <Button 
                type="primary" 
                icon={<SettingOutlined />}
                onClick={() => navigate(`/clusters/${clusterId}/config-center?tab=argocd`)}
              >
                {t('plugins:argocd.goToConfig')}
              </Button>
              <Button onClick={() => navigate(-1)}>
                {t('plugins:argocd.back')}
              </Button>
            </Space>
          </Empty>
        </Card>
      </div>
    );
  }

  return (
    <div style={{ padding: 24 }}>
      {/* 統計卡片 */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={4}>
          <Card size="small">
            <Statistic title={t('plugins:argocd.totalApps')} value={stats.total} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title={t('plugins:argocd.synced')} value={stats.synced} valueStyle={{ color: '#52c41a' }} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title={t('plugins:argocd.outOfSync')} value={stats.outOfSync} valueStyle={{ color: '#faad14' }} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title={t('plugins:argocd.healthy')} value={stats.healthy} valueStyle={{ color: '#52c41a' }} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title={t('plugins:argocd.degraded')} value={stats.degraded} valueStyle={{ color: '#ff4d4f' }} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
            <Button 
              type="link" 
              icon={<SettingOutlined />}
              onClick={() => navigate(`/clusters/${clusterId}/config-center?tab=argocd`)}
            >
              {t('plugins:argocd.configManagement')}
            </Button>
          </Card>
        </Col>
      </Row>

      {/* 應用列表 */}
      <Card
        title={t('plugins:argocd.title')}
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={loadApplications}>
              {t('common:actions.refresh')}
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalVisible(true)}>
              {t('plugins:argocd.createApp')}
            </Button>
          </Space>
        }
      >
        {applications.length > 0 ? (
          <Table
            columns={columns}
            dataSource={applications}
            rowKey="name"
            loading={loading}
            pagination={{ pageSize: 20, showSizeChanger: true, showTotal: (total) => t('plugins:argocd.totalCount', { total }) }}
            scroll={{ x: 1200 }}
          />
        ) : (
          <Empty 
            description={
              <span>
                {t('plugins:argocd.noApps')}
                <Button type="link" onClick={() => setCreateModalVisible(true)}>
                  {t('plugins:argocd.clickToCreate')}
                </Button>
              </span>
            }
          />
        )}
      </Card>

      {/* 建立應用彈窗 */}
      <ArgoCDAppFormModal
        open={createModalVisible}
        form={form}
        creating={creating}
        onOk={handleCreate}
        onCancel={() => {
          setCreateModalVisible(false);
          form.resetFields();
        }}
      />

      {/* 應用詳情抽屜 */}
      <ArgoCDAppDetailDrawer
        open={detailDrawerVisible}
        selectedApp={selectedApp}
        onClose={() => setDetailDrawerVisible(false)}
        onSync={handleSync}
        onRollback={handleRollback}
      />
    </div>
  );
};

export default ArgoCDApplicationsPage;

