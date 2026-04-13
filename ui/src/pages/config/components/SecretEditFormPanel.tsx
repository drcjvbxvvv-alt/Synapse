/**
 * SecretEditFormPanel — form-mode editing panel for SecretEdit page.
 * Renders the basic info, labels, annotations, and data cards.
 * State lives in the parent (SecretEdit); all changes are via callbacks.
 */
import React from 'react';
import {
  Card,
  Row,
  Col,
  Input,
  Button,
  Space,
  Alert,
  Typography,
} from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import type { SecretDetail } from '../../../services/configService';

const { Text } = Typography;

interface KVItem {
  key: string;
  value: string;
}

interface SecretEditFormPanelProps {
  secret: SecretDetail;
  name: string | undefined;
  namespace: string | undefined;
  formLabels: KVItem[];
  formAnnotations: KVItem[];
  formData: KVItem[];
  onLabelsChange: (items: KVItem[]) => void;
  onAnnotationsChange: (items: KVItem[]) => void;
  onDataChange: (items: KVItem[]) => void;
}

const SecretEditFormPanel: React.FC<SecretEditFormPanelProps> = ({
  secret,
  name,
  namespace,
  formLabels,
  formAnnotations,
  formData,
  onLabelsChange,
  onAnnotationsChange,
  onDataChange,
}) => {

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      {/* Basic info */}
      <Card title="基本資訊">
        <Row gutter={16} style={{ marginBottom: 16 }}>
          <Col span={8}>
            <div style={{ marginBottom: 8 }}>
              <Text strong>名稱</Text>
            </div>
            <Input value={name} disabled />
          </Col>
          <Col span={8}>
            <div style={{ marginBottom: 8 }}>
              <Text strong>命名空間</Text>
            </div>
            <Input value={namespace} disabled />
          </Col>
          <Col span={8}>
            <div style={{ marginBottom: 8 }}>
              <Text strong>類型</Text>
            </div>
            <Input value={secret.type} disabled />
          </Col>
        </Row>
      </Card>

      {/* Labels */}
      <Card
        title="標籤 (Labels)"
        extra={
          <Button
            size="small"
            icon={<PlusOutlined />}
            onClick={() => onLabelsChange([...formLabels, { key: '', value: '' }])}
          >
            新增
          </Button>
        }
      >
        {formLabels.map((item, i) => (
          <Row key={i} gutter={8} style={{ marginBottom: 8 }}>
            <Col span={10}>
              <Input
                placeholder="key"
                value={item.key}
                onChange={e =>
                  onLabelsChange(
                    formLabels.map((p, j) => (j === i ? { ...p, key: e.target.value } : p)),
                  )
                }
              />
            </Col>
            <Col span={10}>
              <Input
                placeholder="value"
                value={item.value}
                onChange={e =>
                  onLabelsChange(
                    formLabels.map((p, j) => (j === i ? { ...p, value: e.target.value } : p)),
                  )
                }
              />
            </Col>
            <Col span={4}>
              <Button
                danger
                icon={<DeleteOutlined />}
                onClick={() => onLabelsChange(formLabels.filter((_, j) => j !== i))}
              />
            </Col>
          </Row>
        ))}
        {formLabels.length === 0 && <Text type="secondary">無標籤</Text>}
      </Card>

      {/* Annotations */}
      <Card
        title="注解 (Annotations)"
        extra={
          <Button
            size="small"
            icon={<PlusOutlined />}
            onClick={() => onAnnotationsChange([...formAnnotations, { key: '', value: '' }])}
          >
            新增
          </Button>
        }
      >
        {formAnnotations.map((item, i) => (
          <Row key={i} gutter={8} style={{ marginBottom: 8 }}>
            <Col span={10}>
              <Input
                placeholder="key"
                value={item.key}
                onChange={e =>
                  onAnnotationsChange(
                    formAnnotations.map((p, j) => (j === i ? { ...p, key: e.target.value } : p)),
                  )
                }
              />
            </Col>
            <Col span={10}>
              <Input
                placeholder="value"
                value={item.value}
                onChange={e =>
                  onAnnotationsChange(
                    formAnnotations.map((p, j) => (j === i ? { ...p, value: e.target.value } : p)),
                  )
                }
              />
            </Col>
            <Col span={4}>
              <Button
                danger
                icon={<DeleteOutlined />}
                onClick={() => onAnnotationsChange(formAnnotations.filter((_, j) => j !== i))}
              />
            </Col>
          </Row>
        ))}
        {formAnnotations.length === 0 && <Text type="secondary">無注解</Text>}
      </Card>

      {/* Data */}
      <Card
        title="資料 (Base64 編碼)"
        extra={
          <Button
            size="small"
            icon={<PlusOutlined />}
            onClick={() => onDataChange([...formData, { key: '', value: '' }])}
          >
            新增
          </Button>
        }
      >
        <Alert
          message="值為 base64 編碼格式"
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
        />
        {formData.map((item, i) => (
          <Row key={i} gutter={8} style={{ marginBottom: 8 }}>
            <Col span={10}>
              <Input
                placeholder="key"
                value={item.key}
                onChange={e =>
                  onDataChange(
                    formData.map((p, j) => (j === i ? { ...p, key: e.target.value } : p)),
                  )
                }
              />
            </Col>
            <Col span={10}>
              <Input.TextArea
                placeholder="value (base64)"
                rows={3}
                value={item.value}
                onChange={e =>
                  onDataChange(
                    formData.map((p, j) => (j === i ? { ...p, value: e.target.value } : p)),
                  )
                }
              />
            </Col>
            <Col span={4}>
              <Button
                danger
                icon={<DeleteOutlined />}
                onClick={() => onDataChange(formData.filter((_, j) => j !== i))}
              />
            </Col>
          </Row>
        ))}
        {formData.length === 0 && <Text type="secondary">無資料</Text>}
      </Card>
    </Space>
  );
};

export default SecretEditFormPanel;
