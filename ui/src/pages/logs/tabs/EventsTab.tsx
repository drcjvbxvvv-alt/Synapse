import React from 'react';
import { Button, Space, Select, Table } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import EmptyState from '../../../components/EmptyState';
import type { EventLogEntry } from '../../../services/logService';
import { getEventColumns } from '../columns';
import { eventTypeOptions } from '../constants';

interface EventsTabProps {
  events: EventLogEntry[];
  eventsLoading: boolean;
  eventNamespace: string;
  setEventNamespace: (v: string) => void;
  eventType: 'Normal' | 'Warning' | undefined;
  setEventType: (v: 'Normal' | 'Warning' | undefined) => void;
  namespaces: string[];
  fetchEvents: () => void;
}

export const EventsTab: React.FC<EventsTabProps> = ({
  events,
  eventsLoading,
  eventNamespace,
  setEventNamespace,
  eventType,
  setEventType,
  namespaces,
  fetchEvents,
}) => {
  const { t } = useTranslation(['logs', 'common']);

  return (
    <div>
      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          placeholder={t('common:table.namespace')}
          allowClear
          style={{ width: 180 }}
          value={eventNamespace || undefined}
          onChange={(v) => setEventNamespace(v || '')}
          showSearch
          options={namespaces.map((ns) => ({ label: ns, value: ns }))}
        />
        <Select
          placeholder={t('logs:events.eventType')}
          allowClear
          style={{ width: 120 }}
          value={eventType}
          onChange={setEventType}
          options={eventTypeOptions}
        />
        <Button
          type="primary"
          icon={<SearchOutlined />}
          onClick={fetchEvents}
          loading={eventsLoading}
        >
          {t('logs:center.query')}
        </Button>
      </Space>

      <Table
        columns={getEventColumns(t)}
        dataSource={events}
        rowKey="id"
        loading={eventsLoading}
        pagination={{
          pageSize: 20,
          showSizeChanger: true,
          showTotal: (total) => t('logs:center.totalCount', { total }),
        }}
        size="small"
        scroll={{ y: 'calc(100vh - 500px)' }}
        locale={{ emptyText: <EmptyState description={t('logs:center.noEvents')} /> }}
      />
    </div>
  );
};
