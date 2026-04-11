import React from 'react';
import { Button, Space, Card, Table, Input, DatePicker, Tag, Tooltip } from 'antd';
import { SearchOutlined, PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
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

  const selectedSource = logSources.find(s => s.id === selectedSrcId);

  return (
    <div>
      {/* Log source management */}
      <Card
        size="small"
        title="日誌源管理"
        style={{ marginBottom: 16 }}
        extra={
          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={onAddSource}>
            新增日誌源
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
            { title: '名稱', dataIndex: 'name' },
            {
              title: '型別',
              dataIndex: 'type',
              width: 130,
              render: (type: string) => (
                <Tag color={type === 'loki' ? 'blue' : 'orange'}>{type.toUpperCase()}</Tag>
              ),
            },
            { title: 'URL', dataIndex: 'url', ellipsis: true },
            {
              title: '狀態',
              dataIndex: 'enabled',
              width: 80,
              render: (v: boolean) => <Tag color={v ? 'green' : 'red'}>{v ? '啟用' : '停用'}</Tag>,
            },
            {
              title: '操作',
              width: 110,
              render: (_: unknown, record: LogSource) => (
                <Space>
                  <Button
                    size="small"
                    icon={<EditOutlined />}
                    type="link"
                    onClick={() => onEditSource(record, srcForm)}
                  />
                  <Tooltip title="刪除">
                    <Button
                      size="small"
                      icon={<DeleteOutlined />}
                      type="link"
                      danger
                      onClick={() => onDeleteSource(record)}
                    />
                  </Tooltip>
                </Space>
              ),
            },
          ]}
        />
      </Card>

      {/* Query interface */}
      <Card size="small" title="查詢日誌" style={{ marginBottom: 16 }}>
        <Space wrap style={{ width: '100%' }}>
          <Input
            placeholder={
              selectedSrcId
                ? selectedSource?.type === 'loki'
                  ? 'LogQL 查詢，如 {namespace="default"}'
                  : 'Lucene 查詢，如 error AND namespace:default'
                : '請先在上方選擇日誌源'
            }
            style={{ width: 420 }}
            value={extQuery}
            onChange={(e) => setExtQuery(e.target.value)}
            onPressEnter={handleExtSearch}
          />
          {selectedSrcId && selectedSource?.type === 'elasticsearch' && (
            <Input
              placeholder="ES Index（如 k8s-logs-*）"
              style={{ width: 180 }}
              value={extIndex}
              onChange={(e) => setExtIndex(e.target.value)}
            />
          )}
          <RangePicker
            showTime
            value={extDateRange}
            onChange={(dates) => setExtDateRange(dates as [Dayjs, Dayjs] | null)}
            placeholder={['開始時間', '結束時間']}
          />
          <Button
            type="primary"
            icon={<SearchOutlined />}
            loading={extSearchLoading}
            onClick={handleExtSearch}
          >
            查詢
          </Button>
        </Space>
      </Card>

      {/* Query results */}
      <Card size="small" title={`查詢結果（${extResults.length} 筆）`}>
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
