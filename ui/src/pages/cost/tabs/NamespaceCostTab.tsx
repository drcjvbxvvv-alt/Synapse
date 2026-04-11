import React from 'react';
import { Button, Space, Row, Col, Card, Table, DatePicker } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip as RechartTooltip, ResponsiveContainer,
} from 'recharts';
import EmptyState from '../../../components/EmptyState';
import type { CostItem } from '../../../services/costService';
import { getNamespaceCostColumns } from '../columns';
import { BAR_PROPS, GRID_STYLE, TOOLTIP_STYLE } from '../constants';

interface NamespaceCostTabProps {
  nsCosts: CostItem[];
  nsLoading: boolean;
  month: string;
  currency: string;
  onMonthChange: (month: string) => void;
  onRefresh: () => void;
}

export const NamespaceCostTab: React.FC<NamespaceCostTabProps> = ({
  nsCosts,
  nsLoading,
  month,
  currency,
  onMonthChange,
  onRefresh,
}) => {
  const { t } = useTranslation(['cost', 'common']);

  const barData = nsCosts.slice(0, 10).map(item => ({
    name: item.name,
    cost: Number(item.est_cost.toFixed(4)),
  }));

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <DatePicker.MonthPicker
          value={dayjs(month, 'YYYY-MM')}
          onChange={v => v && onMonthChange(v.format('YYYY-MM'))}
          allowClear={false}
          style={{ width: 130 }}
        />
        <Button icon={<ReloadOutlined />} onClick={onRefresh}>
          {t('common:actions.refresh')}
        </Button>
      </Space>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col xs={24} lg={14}>
          <Card title={t('cost:tabs.namespaces')} size="small">
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={barData} margin={{ top: 5, right: 20, left: 10, bottom: 60 }}>
                <CartesianGrid {...GRID_STYLE} />
                <XAxis dataKey="name" angle={-30} textAnchor="end" interval={0} tick={{ fontSize: 11 }} />
                <YAxis />
                <RechartTooltip {...TOOLTIP_STYLE} formatter={(v) => [`${currency} ${v}`, t('cost:table.estCost')]} />
                <Bar dataKey="cost" fill="#5B8FF9" {...BAR_PROPS} />
              </BarChart>
            </ResponsiveContainer>
          </Card>
        </Col>
      </Row>

      <Table
        rowKey="name"
        columns={getNamespaceCostColumns(t)}
        dataSource={nsCosts}
        loading={nsLoading}
        size="small"
        scroll={{ x: 800 }}
        pagination={false}
        locale={{ emptyText: <EmptyState description={t('cost:noNamespaces')} /> }}
      />
    </div>
  );
};
