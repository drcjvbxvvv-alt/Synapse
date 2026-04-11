import React from 'react';
import { Button, Space, Table, Alert, Empty } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { WasteItem } from '../../../services/costService';
import { getWasteColumns } from '../columns';

interface WasteTabProps {
  waste: WasteItem[];
  wasteLoading: boolean;
  onRefresh: () => void;
}

export const WasteTab: React.FC<WasteTabProps> = ({
  waste,
  wasteLoading,
  onRefresh,
}) => {
  const { t } = useTranslation(['cost', 'common']);

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={onRefresh}>
          {t('common:actions.refresh')}
        </Button>
      </Space>

      {waste.length === 0 && !wasteLoading ? (
        <Empty description={t('cost:waste.empty')} />
      ) : (
        <>
          <Alert
            type="warning"
            showIcon
            message={t('cost:waste.suggestion')}
            style={{ marginBottom: 12 }}
          />
          <Table
            rowKey={(r: WasteItem) => `${r.namespace}/${r.workload}`}
            columns={getWasteColumns(t)}
            dataSource={waste}
            loading={wasteLoading}
            size="small"
            scroll={{ x: 900 }}
            pagination={false}
          />
        </>
      )}
    </div>
  );
};
