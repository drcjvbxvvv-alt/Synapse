import React, { useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Button, Space, Row, Col, Statistic, Badge, Typography, Tabs } from 'antd';
import {
  AlertOutlined,
  ReloadOutlined,
  SettingOutlined,
  FireOutlined,
  StopOutlined,
  ExclamationCircleOutlined,
  ArrowLeftOutlined,
} from '@ant-design/icons';
import type { TabsProps } from 'antd';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import 'dayjs/locale/zh-cn';
import { useTranslation } from 'react-i18next';
import { usePermission } from '../../hooks/usePermission';
import PageSkeleton from '../../components/PageSkeleton';
import EmptyState from '../../components/EmptyState';
import ReceiverManagement from './ReceiverManagement';
import { useAlertCenter } from './hooks/useAlertCenter';
import { createAlertColumns, createSilenceColumns } from './columns';
import SilenceModal from './components/SilenceModal';
import AlertsTab from './components/AlertsTab';
import SilencesTab from './components/SilencesTab';

dayjs.extend(relativeTime);
dayjs.locale('zh-cn');

const { Title } = Typography;

const AlertCenter: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation(['alert', 'common']);
  const { canDelete } = usePermission();

  const state = useAlertCenter(clusterId);

  const alertColumns = useMemo(
    () => createAlertColumns({ t, onSilence: state.handleOpenSilenceModal }),
    [t, state.handleOpenSilenceModal],
  );

  const silenceColumns = useMemo(
    () => createSilenceColumns({ t, onDelete: state.handleDeleteSilence, canDelete }),
    [t, state.handleDeleteSilence, canDelete],
  );

  const tabItems: TabsProps['items'] = [
    {
      key: 'alerts',
      label: (
        <span>
          <AlertOutlined />
          告警列表
          {state.stats && state.stats.firing > 0 && (
            <Badge count={state.stats.firing} style={{ marginLeft: 8 }} />
          )}
        </span>
      ),
      children: (
        <AlertsTab
          columns={alertColumns}
          filteredAlerts={state.filteredAlerts}
          loading={state.loading}
          searchText={state.searchText}
          setSearchText={state.setSearchText}
          severityFilter={state.severityFilter}
          setSeverityFilter={state.setSeverityFilter}
          statusFilter={state.statusFilter}
          setStatusFilter={state.setStatusFilter}
          onRefresh={state.handleRefresh}
        />
      ),
    },
    {
      key: 'silences',
      label: (
        <span>
          <StopOutlined />
          靜默規則
          {state.silences.filter((s) => s.status.state === 'active').length > 0 && (
            <Badge
              count={state.silences.filter((s) => s.status.state === 'active').length}
              style={{ marginLeft: 8, backgroundColor: '#faad14' }}
            />
          )}
        </span>
      ),
      children: (
        <SilencesTab
          columns={silenceColumns}
          silences={state.silences}
          loading={state.loading}
          onCreateSilence={() => state.handleOpenSilenceModal()}
          onRefresh={state.loadSilences}
        />
      ),
    },
    {
      key: 'receivers',
      label: (
        <span>
          <SettingOutlined />
          告警渠道
        </span>
      ),
      children: clusterId ? <ReceiverManagement clusterId={clusterId} /> : null,
    },
  ];

  if (state.configLoading) return <PageSkeleton variant="table" />;

  if (!state.configEnabled) {
    return (
      <div style={{ padding: 24 }}>
        <Card>
          <EmptyState
            type="not-configured"
            title={t('alert:center.notConfigured')}
            description={t('alert:center.notConfiguredDesc')}
            actions={[
              {
                label: t('alert:center.goToConfig'),
                icon: <SettingOutlined />,
                onClick: state.goToConfig,
              },
              {
                label: t('common:actions.back'),
                icon: <ArrowLeftOutlined />,
                type: 'default',
                onClick: () => navigate(-1),
              },
            ]}
          />
        </Card>
      </div>
    );
  }

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={
          <Space>
            <AlertOutlined />
            <Title level={4} style={{ margin: 0 }}>
              {t('alert:center.title')}
            </Title>
          </Space>
        }
        extra={
          <Space>
            <Button icon={<SettingOutlined />} onClick={state.goToConfig}>
              {t('alert:center.config')}
            </Button>
            <Button icon={<ReloadOutlined />} onClick={state.handleRefresh}>
              {t('common:actions.refresh')}
            </Button>
          </Space>
        }
      >
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          <Col xs={12} sm={6}>
            <Card hoverable>
              <Statistic
                title={t('alert:center.totalAlerts')}
                value={state.stats?.total || 0}
                prefix={<AlertOutlined style={{ color: '#1890ff' }} />}
              />
            </Card>
          </Col>
          <Col xs={12} sm={6}>
            <Card hoverable>
              <Statistic
                title={t('alert:center.firing')}
                value={state.stats?.firing || 0}
                prefix={<FireOutlined style={{ color: '#ff4d4f' }} />}
                valueStyle={{ color: '#ff4d4f' }}
              />
            </Card>
          </Col>
          <Col xs={12} sm={6}>
            <Card hoverable>
              <Statistic
                title={t('alert:center.suppressed')}
                value={state.stats?.suppressed || 0}
                prefix={<StopOutlined style={{ color: '#faad14' }} />}
                valueStyle={{ color: '#faad14' }}
              />
            </Card>
          </Col>
          <Col xs={12} sm={6}>
            <Card hoverable>
              <Statistic
                title={t('alert:center.criticalAlerts')}
                value={state.stats?.bySeverity?.critical || 0}
                prefix={<ExclamationCircleOutlined style={{ color: '#ff4d4f' }} />}
                valueStyle={{ color: '#ff4d4f' }}
              />
            </Card>
          </Col>
        </Row>

        <Tabs defaultActiveKey="alerts" items={tabItems} />
      </Card>

      <SilenceModal
        open={state.silenceModalVisible}
        form={state.silenceForm}
        t={t}
        onOk={state.handleCreateSilence}
        onCancel={() => state.setSilenceModalVisible(false)}
      />
    </div>
  );
};

export default AlertCenter;
