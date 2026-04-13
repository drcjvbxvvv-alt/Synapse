import React, { useState, useEffect, useCallback } from 'react';
import { Table, Tag, App } from 'antd';
import { CheckCircleOutlined, QuestionCircleOutlined } from '@ant-design/icons';
import EmptyState from '@/components/EmptyState';
import NotInstalledCard from '@/components/NotInstalledCard';
import { useTranslation } from 'react-i18next';
import { gatewayService } from '../../services/gatewayService';
import type { GatewayClassItem, GatewayTabProps } from './gatewayTypes';

const GatewayClassList: React.FC<GatewayTabProps> = ({ clusterId, onCountChange }) => {
  const { message } = App.useApp();
  const { t } = useTranslation(['network', 'common']);
  const [items, setItems] = useState<GatewayClassItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [installed, setInstalled] = useState<boolean | null>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const status = await gatewayService.getStatus(clusterId);
      setInstalled(status.available);
      if (!status.available) {
        onCountChange?.(0);
        return;
      }
      const res = await gatewayService.listGatewayClasses(clusterId);
      setItems(res.items ?? []);
      onCountChange?.(res.total ?? 0);
    } catch {
      message.error(t('network:gatewayapi.messages.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t, onCountChange]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const statusTag = (status: string) => {
    if (status === 'Accepted') {
      return (
        <Tag icon={<CheckCircleOutlined />} color="success">
          {t('network:gatewayapi.status.accepted')}
        </Tag>
      );
    }
    return (
      <Tag icon={<QuestionCircleOutlined />} color="default">
        {status}
      </Tag>
    );
  };

  const columns = [
    {
      title: t('network:gatewayapi.columns.name'),
      dataIndex: 'name',
      key: 'name',
      sorter: (a: GatewayClassItem, b: GatewayClassItem) => a.name.localeCompare(b.name),
    },
    {
      title: t('network:gatewayapi.columns.controller'),
      dataIndex: 'controller',
      key: 'controller',
      render: (v: string) => <code style={{ fontSize: 12 }}>{v}</code>,
    },
    {
      title: t('network:gatewayapi.columns.status'),
      dataIndex: 'acceptedStatus',
      key: 'acceptedStatus',
      render: (v: string) => statusTag(v),
    },
    {
      title: t('network:gatewayapi.columns.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '-',
    },
  ];

  if (installed === false) {
    return (
      <NotInstalledCard
        title={t('network:gatewayapi.notInstalled')}
        description={t('network:gatewayapi.notInstalledDesc')}
        command={t('network:gatewayapi.installCmd')}
        docsUrl="https://gateway-api.sigs.k8s.io/guides/"
        onRecheck={() => {
          setInstalled(null);
          loadData();
        }}
        recheckLoading={loading}
      />
    );
  }

  return (
    <Table
      rowKey="name"
      loading={loading}
      dataSource={items}
      columns={columns}
      pagination={{
        pageSize: 20,
        showTotal: (total) => t('network:gatewayapi.pagination.gatewayClassTotal', { total }),
      }}
      size="middle"
      locale={{ emptyText: <EmptyState description={t('common:messages.noData')} /> }}
    />
  );
};

export default GatewayClassList;
