import React from 'react';
import { Card, Typography } from 'antd';
import type { PermissionTypeInfo } from '../../../types';

const { Title, Paragraph } = Typography;

interface PermissionTypeCardProps {
  type: PermissionTypeInfo;
  selected: boolean;
  onClick: () => void;
}

export const PermissionTypeCard: React.FC<PermissionTypeCardProps> = ({ type, selected, onClick }) => {
  return (
    <Card
      hoverable
      onClick={onClick}
      style={{
        cursor: 'pointer',
        borderColor: selected ? '#1890ff' : undefined,
        backgroundColor: selected ? '#e6f7ff' : undefined,
        height: '100%',
      }}
      styles={{ body: { padding: '12px' } }}
    >
      <Title level={5} style={{ marginBottom: 6, fontSize: 13 }}>
        {type.name}
      </Title>
      <Paragraph
        type="secondary"
        style={{ marginBottom: 0, fontSize: 12 }}
      >
        {type.description}
      </Paragraph>
    </Card>
  );
};
