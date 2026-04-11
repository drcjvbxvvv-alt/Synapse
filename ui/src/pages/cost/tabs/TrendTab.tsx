import React from 'react';
import { Button, Space, Card } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid,
  Tooltip as RechartTooltip, Legend, ResponsiveContainer,
} from 'recharts';
import type { TrendPoint } from '../../../services/costService';
import { COLORS } from '../constants';

interface TrendTabProps {
  trend: TrendPoint[];
  trendLoading: boolean;
  onRefresh: () => void;
}

export const TrendTab: React.FC<TrendTabProps> = ({
  trend,
  trendLoading,
  onRefresh,
}) => {
  const { t } = useTranslation(['cost', 'common']);

  // Build line chart data
  const allNs = Array.from(new Set(trend.flatMap(p => p.breakdown?.map(b => b.namespace) ?? [])));
  const lineData = trend.map(p => {
    const point: Record<string, string | number> = { month: p.month };
    allNs.forEach(ns => {
      point[ns] = p.breakdown?.find(b => b.namespace === ns)?.cost ?? 0;
    });
    point.total = Number(p.total.toFixed(4));
    return point;
  });

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={onRefresh}>
          {t('common:actions.refresh')}
        </Button>
      </Space>

      <Card title={t('cost:trend.title')} loading={trendLoading}>
        <ResponsiveContainer width="100%" height={320}>
          <LineChart data={lineData} margin={{ top: 5, right: 20, left: 10, bottom: 5 }}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="month" />
            <YAxis />
            <RechartTooltip />
            <Legend />
            {allNs.map((ns, i) => (
              <Line
                key={ns}
                type="monotone"
                dataKey={ns}
                stroke={COLORS[i % COLORS.length]}
                dot={false}
              />
            ))}
            <Line type="monotone" dataKey="total" stroke="#000" strokeWidth={2} dot={false} name={t('cost:trend.totalLabel')} />
          </LineChart>
        </ResponsiveContainer>
      </Card>
    </div>
  );
};
