import React from 'react';
import { useParams } from 'react-router-dom';
import { Card, Tabs, Row, Col, Statistic, Space, Tag, Button, Spin } from 'antd';
import {
  FileTextOutlined,
  ThunderboltOutlined,
  SearchOutlined,
  WarningOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  DatabaseOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

import { useLogCenter } from './hooks/useLogCenter';
import { StreamTab, EventsTab, SearchTab, ExternalLogTab } from './tabs';
import { PodSelectorModal, LogSourceModal } from './components';

const LogCenter: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const { t } = useTranslation(['logs', 'common']);
  const state = useLogCenter(clusterId);

  const tabItems = [
    {
      key: 'stream',
      label: (
        <span>
          <ThunderboltOutlined /> {t('logs:tabs.realtime', '實時日誌')}
        </span>
      ),
      children: (
        <StreamTab
          streaming={state.streaming}
          logs={state.logs}
          targets={state.targets}
          showTimestamp={state.showTimestamp}
          setShowTimestamp={state.setShowTimestamp}
          showSource={state.showSource}
          setShowSource={state.setShowSource}
          autoScroll={state.autoScroll}
          setAutoScroll={state.setAutoScroll}
          levelFilter={state.levelFilter}
          setLevelFilter={state.setLevelFilter}
          logSearchKeyword={state.logSearchKeyword}
          setLogSearchKeyword={state.setLogSearchKeyword}
          filteredLogs={state.filteredLogs}
          logsEndRef={state.logsEndRef}
          toggleStream={state.toggleStream}
          clearLogs={state.clearLogs}
          downloadLogs={state.downloadLogs}
          removeTarget={state.removeTarget}
          openPodSelector={state.openPodSelector}
        />
      ),
    },
    {
      key: 'events',
      label: (
        <span>
          <WarningOutlined /> {t('logs:tabs.events', 'K8s事件')}
        </span>
      ),
      children: (
        <EventsTab
          events={state.events}
          eventsLoading={state.eventsLoading}
          eventNamespace={state.eventNamespace}
          setEventNamespace={state.setEventNamespace}
          eventType={state.eventType}
          setEventType={state.setEventType}
          namespaces={state.namespaces}
          fetchEvents={state.fetchEvents}
        />
      ),
    },
    {
      key: 'search',
      label: (
        <span>
          <SearchOutlined /> {t('logs:tabs.search', '日誌搜尋')}
        </span>
      ),
      children: (
        <SearchTab
          searchResults={state.searchResults}
          searchLoading={state.searchLoading}
          searchKeyword={state.searchKeyword}
          setSearchKeyword={state.setSearchKeyword}
          searchNamespaces={state.searchNamespaces}
          setSearchNamespaces={state.setSearchNamespaces}
          searchLevels={state.searchLevels}
          setSearchLevels={state.setSearchLevels}
          searchDateRange={state.searchDateRange}
          setSearchDateRange={state.setSearchDateRange}
          namespaces={state.namespaces}
          handleSearch={state.handleSearch}
        />
      ),
    },
    {
      key: 'external',
      label: (
        <span>
          <DatabaseOutlined /> {t('logs:tabs.external', '外部日誌')}
        </span>
      ),
      children: (
        <ExternalLogTab
          logSources={state.logSources}
          logSourcesLoading={state.logSourcesLoading}
          selectedSrcId={state.selectedSrcId}
          setSelectedSrcId={state.setSelectedSrcId}
          extQuery={state.extQuery}
          setExtQuery={state.setExtQuery}
          extIndex={state.extIndex}
          setExtIndex={state.setExtIndex}
          extDateRange={state.extDateRange}
          setExtDateRange={state.setExtDateRange}
          extResults={state.extResults}
          extSearchLoading={state.extSearchLoading}
          handleExtSearch={state.handleExtSearch}
          onAddSource={() => {
            state.setEditingSrc(null);
            state.srcForm.resetFields();
            state.setSrcModalOpen(true);
          }}
          onEditSource={(src) => {
            state.setEditingSrc(src);
            state.srcForm.setFieldsValue({
              type: src.type,
              name: src.name,
              url: src.url,
              username: src.username,
              enabled: src.enabled,
            });
            state.setSrcModalOpen(true);
          }}
          onDeleteSource={state.handleDeleteLogSource}
          srcForm={state.srcForm}
        />
      ),
    },
  ];

  return (
    <div style={{ padding: 24, background: '#f0f2f5', minHeight: '100vh' }}>
      {/* Stats overview */}
      <Spin spinning={state.statsLoading}>
        <Row gutter={16} style={{ marginBottom: 16 }}>
          <Col span={4}>
            <Card size="small" variant="borderless">
              <Statistic
                title={t('logs:center.totalCount1h')}
                value={state.stats?.total_count || 0}
                prefix={<FileTextOutlined style={{ color: '#1890ff' }} />}
              />
            </Card>
          </Col>
          <Col span={4}>
            <Card size="small" variant="borderless">
              <Statistic
                title={t('logs:center.errorEvents')}
                value={state.stats?.error_count || 0}
                valueStyle={{ color: '#ff4d4f' }}
                prefix={<CloseCircleOutlined />}
              />
            </Card>
          </Col>
          <Col span={4}>
            <Card size="small" variant="borderless">
              <Statistic
                title={t('logs:center.warningEvents')}
                value={state.stats?.warn_count || 0}
                valueStyle={{ color: '#faad14' }}
                prefix={<WarningOutlined />}
              />
            </Card>
          </Col>
          <Col span={12}>
            <Card size="small" variant="borderless">
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ fontWeight: 500 }}>{t('logs:center.namespaceDistribution')}</span>
                <Space wrap size="small">
                  {state.stats?.namespace_stats?.slice(0, 5).map((ns) => (
                    <Tag key={ns.namespace} color="blue">
                      {ns.namespace}: {ns.count}
                    </Tag>
                  ))}
                </Space>
              </div>
            </Card>
          </Col>
        </Row>
      </Spin>

      {/* Main content */}
      <Card variant="borderless">
        <Tabs
          activeKey={state.activeTab}
          onChange={state.setActiveTab}
          items={tabItems}
          tabBarExtraContent={
            <Space>
              <Button icon={<SyncOutlined />} onClick={state.fetchStats}>
                {t('logs:center.refreshStats')}
              </Button>
            </Space>
          }
        />
      </Card>

      {/* Pod selector modal */}
      <PodSelectorModal
        visible={state.podSelectorVisible}
        onOk={state.confirmPodSelection}
        onCancel={() => state.setPodSelectorVisible(false)}
        namespaces={state.namespaces}
        selectedNamespace={state.selectedNamespace}
        setSelectedNamespace={state.setSelectedNamespace}
        pods={state.pods}
        podsLoading={state.podsLoading}
        selectedPods={state.selectedPods}
        setSelectedPods={state.setSelectedPods}
        podSearchKeyword={state.podSearchKeyword}
        setPodSearchKeyword={state.setPodSearchKeyword}
        filteredPods={state.filteredPods}
        selectedPodsSet={state.selectedPodsSet}
        fetchPods={state.fetchPods}
      />

      {/* Log source modal */}
      <LogSourceModal
        visible={state.srcModalOpen}
        editingSource={state.editingSrc}
        form={state.srcForm}
        onOk={state.handleSaveLogSource}
        onCancel={() => {
          state.setSrcModalOpen(false);
          state.setEditingSrc(null);
          state.srcForm.resetFields();
        }}
      />
    </div>
  );
};

export default LogCenter;
