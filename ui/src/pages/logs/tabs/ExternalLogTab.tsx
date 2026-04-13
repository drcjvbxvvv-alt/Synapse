import React from 'react';
import { Button, Space, Card, Table, Input, DatePicker, Tag, Tooltip } from 'antd';
import { SearchOutlined, PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { usePermission } from '../../../hooks/usePermission';
import EmptyState from '../../../components/EmptyState';
import type { Dayjs } from 'dayjs';
import type { FormInstance } from 'antd';
import type { LogEntry, LogSource } from '../../../services/logService';
import { getExternalLogColumns } from '../columns';

const { RangePicker } = DatePicker;

interface ExternalLogTabProps {
  logSources: LogSource[];
  logSourcesLoading: boolean;
  selectedSrcId: number | null;
  setSelectedSrcId: (id: number | null) => void;
  extQuery: string;
  setExtQuery: (v: string) => void;
  extIndex: string;
  setExtIndex: (v: string) => void;
  extDateRange: [Dayjs, Dayjs] | null;
  setExtDateRange: (v: [Dayjs, Dayjs] | null) => void;
  extResults: LogEntry[];
  extSearchLoading: boolean;
  handleExtSearch: () => void;
  onAddSource: () => void;
  onEditSource: (src: LogSource, form: FormInstance) => void;
  onDeleteSource: (src: LogSource) => void;
  srcForm: FormInstance;
}

export const ExternalLogTab: React.FC<ExternalLogTabProps> = ({
  logSources,
  logSourcesLoading,
  selectedSrcId,
  setSelectedSrcId,
  extQuery,
  setExtQuery,
  extIndex,
  setExtIndex,
  extDateRange,
  setExtDateRange,
  extResults,
  extSearchLoading,
  handleExtSearch,
  onAddSource,
  onEditSource,
  onDeleteSource,
  srcForm,
}) => {
  const { t } = useTranslation(['logs', 'common']);
  const { canDelete } = usePermission();

  const selectedSource = logSources.find(s => s.id === selectedSrcId);

  return (
    <div>
      {/* Log source management */}
      <Card
        size="small"
        title={t('logs:center.logSourceManagement')}
        style={{ marginBottom: 16 }}
        extra={
          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={onAddSource}>
            {t('logs:center.addLogSource')}
          </Button>
        }
      >
        <Table<LogSource>
          scroll={{ x: 'max-content' }}
          loading={logSourcesLoading}
          dataSource={logSources}
          rowKey="id"
          size="small"
          pagination={false}
          locale={{ emptyText: <EmptyState description={t('logs:center.noLogSources')} /> }}
          rowSelection={{
            type: 'radio',
            selectedRowKeys: selectedSrcId ? [selectedSrcId] : [],
            onChange: (keys) => setSelectedSrcId(keys[0] as number),
          }}
          columns={[
            { title: t('logs:center.name'), dataIndex: 'name', ellipsis: true },
            {
              title: t('logs:center.type'),
              dataIndex: 'type',
              width: 130,
              render: (type: string) => (
                <Tag color={type === 'loki' ? 'blue' : 'orange'}>{type.toUpperCase()}</Tag>
              ),
            },
            { title: t('logs:center.url'), dataIndex: 'url', ellipsis: true, width: 200 },
            {
              title: t('logs:center.status'),
              dataIndex: 'enabled',
              width: 80,
              render: (v: boolean) => <Tag color={v ? 'green' : 'red'}>{v ? t('logs:center.enabled') : t('logs:center.disabled')}</Tag>,
            },
            {
              title: t('logs:center.actions'),
              width: 110,
              render: (_: unknown, record: LogSource) => (
                <Space>
                  <Button
                    size="small"
                    icon={<EditOutlined />}
                    type="link"
                    onClick={() => onEditSource(record, srcForm)}
                  />
                  {canDelete() && (
                    <Tooltip title={t('logs:center.delete')}>
                      <Button
                        size="small"
                        icon={<DeleteOutlined />}
                        type="link"
                        danger
                        onClick={() => onDeleteSource(record)}
                      />
                    </Tooltip>
                  )}
                </Space>
              ),
            },
          ]}
        />
      </Card>

      {/* Query interface */}
      <Card size="small" title={t('logs:center.queryLogs')} style={{ marginBottom: 16 }}>
        <Space wrap style={{ width: '100%' }}>
          <Input
            placeholder={
              selectedSrcId
                ? selectedSource?.type === 'loki'
                  ? t('logs:center.logQLQuery')
                  : t('logs:center.luceneQuery')
                : t('logs:center.selectLogSourceFirst')
            }
            style={{ width: 420 }}
            value={extQuery}
            onChange={(e) => setExtQuery(e.target.value)}
            onPressEnter={handleExtSearch}
          />
          {selectedSrcId && selectedSource?.type === 'elasticsearch' && (
            <Input
              placeholder={t('logs:center.esIndex')}
              style={{ width: 180 }}
              value={extIndex}
              onChange={(e) => setExtIndex(e.target.value)}
            />
          )}
          <RangePicker
            showTime
            value={extDateRange}
            onChange={(dates) => setExtDateRange(dates as [Dayjs, Dayjs] | null)}
            placeholder={[t('logs:center.startTime'), t('logs:center.endTime')]}
          />
          <Button
            type="primary"
            icon={<SearchOutlined />}
            loading={extSearchLoading}
            onClick={handleExtSearch}
          >
            {t('logs:center.query')}
          </Button>
        </Space>
      </Card>

      {/* Query results */}
      <Card size="small" title={t('logs:center.queryResults', { count: extResults.length })}>
        <Table<LogEntry>
          dataSource={extResults}
          rowKey={(r, i) => r.id || String(i)}
          size="small"
          loading={extSearchLoading}
          pagination={{ pageSize: 50, showSizeChanger: true }}
          scroll={{ y: 'calc(100vh - 600px)' }}
          columns={getExternalLogColumns(t)}
          locale={{ emptyText: <EmptyState description={t('logs:center.noExternalLogResults')} /> }}
        />
      </Card>
    </div>
  );
};
