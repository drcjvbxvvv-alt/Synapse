import { Button, Dropdown, Popconfirm, Space, Tooltip } from 'antd';
import { MoreOutlined } from '@ant-design/icons';
import { useState } from 'react';
import type { ReactNode } from 'react';

interface ActionItem {
  key: string;
  label: string;
  icon: ReactNode;
  danger?: boolean;
  onClick?: () => void;
  confirm?: { title: string; description: string };
}

interface ActionButtonsProps {
  primary?: ActionItem[];   // 常駐，最多 2 個，純 icon + Tooltip
  more?: ActionItem[];      // 收進下拉，danger 項自動排最後加分隔線
}

export function ActionButtons({ primary = [], more = [] }: ActionButtonsProps) {
  const [pendingConfirm, setPendingConfirm] = useState<ActionItem | null>(null);

  const dangerItems = more.filter(i => i.danger);
  const normalItems = more.filter(i => !i.danger);

  const toMenuItem = (i: ActionItem) => ({
    key: i.key,
    label: i.label,
    icon: i.icon,
    danger: i.danger,
    onClick: i.confirm ? () => setPendingConfirm(i) : i.onClick,
  });

  const menuItems = [
    ...normalItems.map(toMenuItem),
    ...(dangerItems.length ? [{ type: 'divider' as const }] : []),
    ...dangerItems.map(toMenuItem),
  ];

  return (
    <Space size={0}>
      {primary.map(item => {
        const btn = (
          <Button
            key={item.key}
            type="link"
            size="small"
            icon={item.icon}
            danger={item.danger}
            onClick={item.onClick}
          />
        );
        if (item.confirm) {
          return (
            <Popconfirm
              key={item.key}
              title={item.confirm.title}
              description={item.confirm.description}
              onConfirm={item.onClick}
              okButtonProps={{ danger: true }}
            >
              <Tooltip title={item.label}>{btn}</Tooltip>
            </Popconfirm>
          );
        }
        return <Tooltip key={item.key} title={item.label}>{btn}</Tooltip>;
      })}
      {more.length > 0 && (
        <>
          <Dropdown menu={{ items: menuItems }} trigger={['click']}>
            <Tooltip title="更多操作">
              <Button type="link" size="small" icon={<MoreOutlined />} />
            </Tooltip>
          </Dropdown>
          {pendingConfirm?.confirm && (
            <Popconfirm
              open
              title={pendingConfirm.confirm.title}
              description={pendingConfirm.confirm.description}
              onConfirm={() => { pendingConfirm.onClick?.(); setPendingConfirm(null); }}
              onCancel={() => setPendingConfirm(null)}
              okButtonProps={{ danger: true }}
            >
              <span />
            </Popconfirm>
          )}
        </>
      )}
    </Space>
  );
}
