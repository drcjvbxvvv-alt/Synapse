import React from 'react';
import { Table, Button, Space, Row, Col, Typography } from 'antd';
import { ReloadOutlined, PlusOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import type { Silence } from '../../../services/alertService';

const { Text } = Typography;

interface SilencesTabProps {
  columns: ColumnsType<Silence>;
  silences: Silence[];
  loading: boolean;
  onCreateSilence: () => void;
  onRefresh: () => void;
}

const SilencesTab: React.FC<SilencesTabProps> = ({
  columns,
  silences,
  loading,
  onCreateSilence,
  onRefresh,
}) => {
  const { t } = useTranslation(['alert', 'common']);

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col flex="auto">
          <Text type="secondary">{t('alert:center.silenceRulesDesc')}</Text>
        </Col>
        <Col>
          <Space>
            <Button type="primary" icon={<PlusOutlined />} onClick={onCreateSilence}>
              {t('alert:center.createSilenceRule')}
            </Button>
            <Button icon={<ReloadOutlined />} onClick={onRefresh}>
              {t('common:actions.refresh')}
            </Button>
          </Space>
        </Col>
      </Row>
      <Table
        scroll={{ x: 'max-content' }}
        columns={columns}
        dataSource={silences.filter((s) => s.status.state !== 'expired')}
        rowKey="id"
        loading={loading}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => t('alert:center.totalCountRules', { total }),
        }}
      />
    </div>
  );
};

export default SilencesTab;
