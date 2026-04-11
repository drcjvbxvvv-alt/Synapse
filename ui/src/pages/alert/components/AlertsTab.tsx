import React from 'react';
import { Table, Button, Space, Row, Col, Input, Select, Descriptions } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import type { Alert } from '../../../services/alertService';

const { Search } = Input;
const { Option } = Select;

interface AlertsTabProps {
  columns: ColumnsType<Alert>;
  filteredAlerts: Alert[];
  loading: boolean;
  searchText: string;
  setSearchText: (v: string) => void;
  severityFilter: string;
  setSeverityFilter: (v: string) => void;
  statusFilter: string;
  setStatusFilter: (v: string) => void;
  onRefresh: () => void;
}

const AlertsTab: React.FC<AlertsTabProps> = ({
  columns,
  filteredAlerts,
  loading,
  searchText,
  setSearchText,
  severityFilter,
  setSeverityFilter,
  statusFilter,
  setStatusFilter,
  onRefresh,
}) => {
  const { t } = useTranslation(['alert', 'common']);

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col flex="auto">
          <Space>
            <Search
              placeholder={t('alert:center.searchPlaceholder')}
              allowClear
              style={{ width: 300 }}
              value={searchText}
              onSearch={setSearchText}
              onChange={(e) => setSearchText(e.target.value)}
            />
            <Select
              placeholder={t('alert:center.severityFilter')}
              allowClear
              style={{ width: 120 }}
              value={severityFilter || undefined}
              onChange={(value) => setSeverityFilter(value || '')}
            >
              <Option value="critical">Critical</Option>
              <Option value="warning">Warning</Option>
              <Option value="info">Info</Option>
            </Select>
            <Select
              placeholder={t('alert:center.statusFilter')}
              allowClear
              style={{ width: 120 }}
              value={statusFilter || undefined}
              onChange={(value) => setStatusFilter(value || '')}
            >
              <Option value="active">{t('alert:center.statusFiring')}</Option>
              <Option value="suppressed">{t('alert:center.statusSuppressed')}</Option>
            </Select>
          </Space>
        </Col>
        <Col>
          <Button icon={<ReloadOutlined />} onClick={onRefresh}>
            {t('common:actions.refresh')}
          </Button>
        </Col>
      </Row>
      <Table
        scroll={{ x: 'max-content' }}
        columns={columns}
        dataSource={filteredAlerts}
        rowKey="fingerprint"
        loading={loading}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => t('alert:center.totalCountAlerts', { total }),
        }}
        expandable={{
          expandedRowRender: (record) => (
            <Descriptions size="small" column={2}>
              <Descriptions.Item label={t('alert:center.instance')}>
                {record.labels.instance || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="Job">{record.labels.job || '-'}</Descriptions.Item>
              <Descriptions.Item label={t('common:table.namespace')}>
                {record.labels.namespace || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="Pod">{record.labels.pod || '-'}</Descriptions.Item>
              <Descriptions.Item label={t('alert:center.detailDescription')} span={2}>
                {record.annotations?.description || '-'}
              </Descriptions.Item>
              {record.status.silencedBy?.length > 0 && (
                <Descriptions.Item label={t('alert:center.silenceRule')} span={2}>
                  {record.status.silencedBy.join(', ')}
                </Descriptions.Item>
              )}
            </Descriptions>
          ),
        }}
      />
    </div>
  );
};

export default AlertsTab;
