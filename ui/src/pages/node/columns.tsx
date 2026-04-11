import React from 'react';
import { Button, Space, Tag, Progress, Tooltip, Badge, Dropdown } from 'antd';
import {
  EyeOutlined,
  MoreOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  CloseCircleOutlined,
  DesktopOutlined,
  CodeOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { Node, NodeTaint } from '../../types';
import type { TFunction } from 'i18next';

interface ColumnParams {
  t: TFunction;
  tc: TFunction;
  sortField: string;
  sortOrder: 'ascend' | 'descend' | null;
  handleViewDetail: (name: string) => void;
  handleNodeTerminal: (name: string) => void;
  handleCordon: (name: string) => void;
  handleUncordon: (name: string) => void;
  handleDrain: (name: string) => void;
}

export const getStatusTag = (status: string, t: TFunction) => {
  switch (status) {
    case 'Ready':
      return <Tag icon={<CheckCircleOutlined />} color="success">{t('status.ready')}</Tag>;
    case 'NotReady':
      return <Tag icon={<CloseCircleOutlined />} color="error">{t('status.notReady')}</Tag>;
    default:
      return <Tag icon={<ExclamationCircleOutlined />} color="default">{t('status.unknown')}</Tag>;
  }
};

export const getStatusIcon = (node: Node) => {
  if (node.status === 'Ready') {
    const hasNoScheduleTaint = node.taints?.some(
      taint => taint.effect === 'NoSchedule' || taint.effect === 'NoExecute'
    );

    if (hasNoScheduleTaint) {
      return <Badge status="warning" />;
    }

    if (node.cpuUsage > 80 || node.memoryUsage > 80) {
      return <Badge status="warning" />;
    }

    return <Badge status="success" />;
  } else if (node.status === 'NotReady') {
    return <Badge status="error" />;
  } else {
    return <Badge status="default" />;
  }
};

export const getRoleTags = (roles: string[]) => {
  return (
    <Space>
      {roles.map(role => {
        const isMaster = role.toLowerCase().includes('master') || role.toLowerCase().includes('control-plane');
        return (
          <Tag key={role} color={isMaster ? 'gold' : 'blue'}>
            {isMaster ? 'M' : 'W'}
          </Tag>
        );
      })}
    </Space>
  );
};

export const getTaintTooltip = (taints: NodeTaint[], t: TFunction) => {
  if (!taints || taints.length === 0) {
    return t('detail.taints') + ': 0';
  }

  return (
    <div>
      <div>{t('detail.taints')}:</div>
      {taints.map((taint, index) => (
        <div key={index}>
          {taint.key}{taint.value ? `=${taint.value}` : ''}:{taint.effect}
        </div>
      ))}
    </div>
  );
};

export const createNodeColumns = (params: ColumnParams): ColumnsType<Node> => {
  const { t, tc, sortField, sortOrder, handleViewDetail, handleNodeTerminal, handleCordon, handleUncordon, handleDrain } = params;

  return [
    {
      title: t('columns.status'),
      key: 'status',
      width: 60,
      render: (_, record) => getStatusIcon(record),
    },
    {
      title: t('columns.name'),
      dataIndex: 'name',
      key: 'name',
      width: 180,
      fixed: 'left' as const,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (text) => (
        <Space style={{ width: '100%' }}>
          <DesktopOutlined style={{ color: '#1890ff', flexShrink: 0 }} />
          <Button
            type="link"
            onClick={() => handleViewDetail(text)}
            style={{
              padding: 0,
              height: 'auto',
              whiteSpace: 'normal',
              wordBreak: 'break-all',
              textAlign: 'left'
            }}
          >
            {text}
          </Button>
        </Space>
      ),
    },
    {
      title: t('columns.roles'),
      key: 'roles',
      width: 80,
      render: (_, record) => getRoleTags(record.roles),
    },
    {
      title: t('columns.version'),
      dataIndex: 'version',
      key: 'version',
      width: 100,
      sorter: true,
      sortOrder: sortField === 'version' ? sortOrder : null,
    },
    {
      title: t('columns.readyStatus'),
      key: 'readyStatus',
      width: 80,
      render: (_, record) => getStatusTag(record.status, t),
    },
    {
      title: t('columns.cpu'),
      key: 'cpuUsage',
      dataIndex: 'cpuUsage',
      width: 120,
      sorter: true,
      sortOrder: sortField === 'cpuUsage' ? sortOrder : null,
      render: (_, record) => (
        <Progress
          percent={Math.round(record.cpuUsage || 0)}
          size="small"
          status={
            record.cpuUsage > 80
              ? 'exception'
              : record.cpuUsage > 60
                ? 'active'
                : 'success'
          }
          format={() => `${(record.cpuUsage || 0).toFixed(1)}%`}
        />
      ),
    },
    {
      title: t('columns.memory'),
      key: 'memoryUsage',
      dataIndex: 'memoryUsage',
      width: 120,
      sorter: true,
      sortOrder: sortField === 'memoryUsage' ? sortOrder : null,
      render: (_, record) => (
        <Progress
          percent={Math.round(record.memoryUsage || 0)}
          size="small"
          status={
            record.memoryUsage > 80
              ? 'exception'
              : record.memoryUsage > 60
                ? 'active'
                : 'success'
          }
          format={() => `${(record.memoryUsage || 0).toFixed(1)}%`}
        />
      ),
    },
    {
      title: t('columns.pods'),
      key: 'podCount',
      width: 100,
      sorter: true,
      sortOrder: sortField === 'podCount' ? sortOrder : null,
      render: (_, record) => `${record.podCount}/${record.maxPods}`,
    },
    {
      title: t('columns.taints'),
      key: 'taints',
      width: 80,
      render: (_, record) => (
        <Tooltip title={getTaintTooltip(record.taints, t)}>
          <Tag color={record.taints?.length ? 'orange' : 'default'}>
            {record.taints?.length || 0}
          </Tag>
        </Tooltip>
      ),
    },
    {
      title: tc('table.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      sorter: true,
      sortOrder: sortField === 'createdAt' ? sortOrder : null,
      render: (text) => {
        if (!text) return '-';
        const date = new Date(text);
        return <span>{date.toLocaleString()}</span>;
      },
    },
    {
      title: tc('table.actions'),
      key: 'actions',
      width: 150,
      fixed: 'right' as const,
      render: (_, record) => (
        <Space>
          <Button
            type="text"
            icon={<EyeOutlined />}
            onClick={() => handleViewDetail(record.name)}
            title={tc('actions.view')}
          />
          <Button
            type="text"
            icon={<CodeOutlined />}
            onClick={() => handleNodeTerminal(record.name)}
            title={t('actions.terminal')}
          />
          <Dropdown
            menu={{
              items: [
                ...(record.taints?.some(t => t.effect === 'NoSchedule') ? [{
                  key: 'uncordon',
                  label: t('actions.uncordon'),
                  onClick: () => handleUncordon(record.name)
                }] : [{
                  key: 'cordon',
                  label: t('actions.cordon'),
                  onClick: () => handleCordon(record.name)
                }]),
                {
                  key: 'drain',
                  label: t('actions.drain'),
                  onClick: () => handleDrain(record.name)
                }
              ]
            }}
          >
            <Button type="text" icon={<MoreOutlined />} />
          </Dropdown>
        </Space>
      ),
    },
  ];
};
