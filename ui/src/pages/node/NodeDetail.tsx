import React, { useState } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Card,
  Typography,
  Button,
  Space,
  Tabs,
  Tag,
  Descriptions,
} from 'antd';
import {
  ReloadOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  DesktopOutlined,
  BarChartOutlined,
  ArrowLeftOutlined,
} from '@ant-design/icons';
import ErrorPage from '../../components/ErrorPage';
import { useNodeDetail } from './hooks/useNodeDetail';
import { createNodeDetailTabItems, LabelModal, TaintModal, DrainModal } from './components';

const { Title, Text } = Typography;

// Helper to get status tag
const getStatusTag = (status: string, t: (key: string) => string) => {
  switch (status) {
    case 'Ready':
      return <Tag icon={<CheckCircleOutlined />} color="success">{t('status.ready')}</Tag>;
    case 'NotReady':
      return <Tag icon={<CloseCircleOutlined />} color="error">{t('status.notReady')}</Tag>;
    default:
      return <Tag icon={<ExclamationCircleOutlined />} color="default">{t('status.unknown')}</Tag>;
  }
};

const NodeDetail: React.FC = () => {
  const { clusterId, nodeName } = useParams<{ clusterId: string; nodeName: string }>();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { t } = useTranslation('node');
  const { t: tc } = useTranslation('common');

  const defaultTab = searchParams.get('tab') || 'overview';
  const [activeTab, setActiveTab] = useState(defaultTab);

  const state = useNodeDetail({ clusterId, nodeName, t, tc });

  // Error state: node not found
  if (!state.node && !state.loading) {
    return (
      <ErrorPage
        status={404}
        title={t('messages.nodeNotFound')}
        subTitle={t('messages.checkNodeName')}
        showHome={false}
        showBack
        onRetry={() => navigate(`/clusters/${clusterId}/nodes`)}
      />
    );
  }

  // Build tab items
  const tabItems = createNodeDetailTabItems({
    clusterId: clusterId || '',
    nodeName: nodeName || '',
    node: state.node,
    pods: state.pods,
    loadingPods: state.loadingPods,
    t,
    tc,
    navigate,
    handleExportPods: state.handleExportPods,
    handleRemoveLabel: state.handleRemoveLabel,
    handleRemoveTaint: state.handleRemoveTaint,
    setLabelModalVisible: state.setLabelModalVisible,
    setTaintModalVisible: state.setTaintModalVisible,
  });

  return (
    <div>
      {/* Page Header */}
      <div className="page-header">
        <div style={{ display: 'flex', alignItems: 'center', marginBottom: 24 }}>
          <Button
            type="text"
            icon={<ArrowLeftOutlined />}
            onClick={() => navigate(`/clusters/${clusterId}/nodes`)}
            style={{ marginRight: 16 }}
          >
            {t('actions.backToList')}
          </Button>
          <div style={{ flex: 1 }}>
            <Title level={2} style={{ margin: 0 }}>
              <DesktopOutlined style={{ marginRight: 8, color: '#1890ff' }} />
              {nodeName}
            </Title>
            <Text type="secondary">{t('detail.subtitle')}</Text>
          </div>
          <Space>
            <Button
              icon={<ReloadOutlined />}
              onClick={state.refreshAllData}
              loading={state.loading}
            >
              {tc('actions.refresh')}
            </Button>
            <Button
              icon={<BarChartOutlined />}
              type="primary"
              onClick={() => setActiveTab('monitoring')}
            >
              {tc('menu.monitoring')}
            </Button>
          </Space>
        </div>
      </div>

      {state.node && (
        <>
          {/* Node Basic Info */}
          <Card style={{ marginBottom: 24 }}>
            <Descriptions title={t('detail.info')} column={3}>
              <Descriptions.Item label={t('columns.name')}>{state.node.name}</Descriptions.Item>
              <Descriptions.Item label={t('detail.kubeletVersion')}>{state.node.kubeletVersion}</Descriptions.Item>
              <Descriptions.Item label={t('columns.status')}>
                {getStatusTag(state.node.status, t)}
              </Descriptions.Item>
              <Descriptions.Item label={t('columns.roles')}>
                <Space>
                  {state.node.roles.map(role => {
                    const isMaster = role.toLowerCase().includes('master') || role.toLowerCase().includes('control-plane');
                    return (
                      <Tag key={role} color={isMaster ? 'gold' : 'blue'}>
                        {role}
                      </Tag>
                    );
                  })}
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label={t('detail.osImage')}>{state.node.osImage}</Descriptions.Item>
              <Descriptions.Item label={t('detail.kernelVersion')}>{state.node.kernelVersion}</Descriptions.Item>
              <Descriptions.Item label={t('detail.containerRuntime')}>{state.node.containerRuntime}</Descriptions.Item>
              <Descriptions.Item label={t('resources.cpuCapacity')}>{state.node.resources?.cpu}m</Descriptions.Item>
              <Descriptions.Item label={t('resources.memoryCapacity')}>{state.node.resources?.memory}Mi</Descriptions.Item>
              <Descriptions.Item label={t('resources.maxPods')}>{state.node.resources?.pods}</Descriptions.Item>
              <Descriptions.Item label={tc('table.createdAt')}>
                {new Date(state.node.creationTimestamp).toLocaleString()}
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </>
      )}

      {/* Tabs */}
      <Card>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={tabItems}
        />
      </Card>

      {/* Modals */}
      <LabelModal
        visible={state.labelModalVisible}
        onCancel={() => state.setLabelModalVisible(false)}
        node={state.node}
        newLabelKey={state.newLabelKey}
        newLabelValue={state.newLabelValue}
        setNewLabelKey={state.setNewLabelKey}
        setNewLabelValue={state.setNewLabelValue}
        handleAddLabel={state.handleAddLabel}
        handleRemoveLabel={state.handleRemoveLabel}
        t={t}
        tc={tc}
      />

      <TaintModal
        visible={state.taintModalVisible}
        onCancel={() => state.setTaintModalVisible(false)}
        newTaintKey={state.newTaintKey}
        newTaintValue={state.newTaintValue}
        newTaintEffect={state.newTaintEffect}
        setNewTaintKey={state.setNewTaintKey}
        setNewTaintValue={state.setNewTaintValue}
        setNewTaintEffect={state.setNewTaintEffect}
        handleAddTaint={state.handleAddTaint}
        t={t}
        tc={tc}
      />

      <DrainModal
        visible={state.drainModalVisible}
        onCancel={() => state.setDrainModalVisible(false)}
        nodeName={nodeName || ''}
        drainOptions={state.drainOptions}
        setDrainOptions={state.setDrainOptions}
        handleDrain={state.handleDrain}
        t={t}
        tc={tc}
      />
    </div>
  );
};

export default NodeDetail;
