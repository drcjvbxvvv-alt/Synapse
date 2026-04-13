/**
 * ConfigMapFormBody — form-mode content for ConfigMapCreate page.
 * Renders basic info, labels, annotations, and data content cards.
 * All state is owned by the parent; changes are communicated via callbacks.
 */
import React from 'react';
import {
  Card,
  Form,
  Input,
  Button,
  Space,
  Row,
  Col,
  Select,
  Tooltip,
} from 'antd';
import type { FormInstance } from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  QuestionCircleOutlined,
} from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import { useTranslation } from 'react-i18next';

interface KVItem {
  key: string;
  value: string;
}

interface DataItem {
  key: string;
  value: string;
}

interface ConfigMapFormBodyProps {
  form: FormInstance;
  name: string;
  namespace: string;
  namespaces: string[];
  loadingNamespaces: boolean;
  labels: KVItem[];
  annotations: KVItem[];
  dataItems: DataItem[];
  onNameChange: (value: string) => void;
  onNamespaceChange: (value: string) => void;
  onAddLabel: () => void;
  onRemoveLabel: (index: number) => void;
  onLabelChange: (index: number, field: 'key' | 'value', value: string) => void;
  onAddAnnotation: () => void;
  onRemoveAnnotation: (index: number) => void;
  onAnnotationChange: (index: number, field: 'key' | 'value', value: string) => void;
  onAddDataItem: () => void;
  onRemoveDataItem: (index: number) => void;
  onDataKeyChange: (index: number, value: string) => void;
  onDataValueChange: (index: number, value: string | undefined) => void;
}

const ConfigMapFormBody: React.FC<ConfigMapFormBodyProps> = ({
  form,
  name,
  namespace,
  namespaces,
  loadingNamespaces,
  labels,
  annotations,
  dataItems,
  onNameChange,
  onNamespaceChange,
  onAddLabel,
  onRemoveLabel,
  onLabelChange,
  onAddAnnotation,
  onRemoveAnnotation,
  onAnnotationChange,
  onAddDataItem,
  onRemoveDataItem,
  onDataKeyChange,
  onDataValueChange,
}) => {
  const { t } = useTranslation(['config', 'common']);

  return (
    <>
      {/* Basic info */}
      <Card title={t('config:create.basicInfo')}>
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                label={t('config:create.name')}
                required
                help={t('config:create.configMapNameHelp')}
              >
                <Input
                  placeholder={t('config:create.configMapNamePlaceholder')}
                  value={name}
                  onChange={e => onNameChange(e.target.value)}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label={t('config:create.namespace')}
                help={t('config:create.configMapNamespaceHelp')}
              >
                <Select
                  value={namespace}
                  onChange={onNamespaceChange}
                  placeholder={t('config:create.namespacePlaceholder')}
                  loading={loadingNamespaces}
                  showSearch
                  filterOption={(input, option) => {
                    if (!option?.children) return false;
                    const text = String(option.children);
                    return text.toLowerCase().includes(input.toLowerCase());
                  }}
                >
                  {namespaces.map(ns => (
                    <Select.Option key={ns} value={ns}>
                      {ns}
                    </Select.Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Card>

      {/* Labels */}
      <Card
        title={
          <Space>
            <span>{t('config:create.labels')}</span>
            <Tooltip title={t('config:create.labelsTooltip')}>
              <QuestionCircleOutlined style={{ color: '#999' }} />
            </Tooltip>
          </Space>
        }
        extra={
          <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={onAddLabel}>
            {t('config:create.addLabel')}
          </Button>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }} size="small">
          {labels.map((label, index) => (
            <Row key={index} gutter={8} align="middle">
              <Col span={10}>
                <Input
                  placeholder={t('config:create.keyPlaceholder')}
                  value={label.key}
                  onChange={e => onLabelChange(index, 'key', e.target.value)}
                />
              </Col>
              <Col span={10}>
                <Input
                  placeholder={t('config:create.valuePlaceholder')}
                  value={label.value}
                  onChange={e => onLabelChange(index, 'value', e.target.value)}
                />
              </Col>
              <Col span={4}>
                <Button
                  type="text"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() => onRemoveLabel(index)}
                >
                  {t('common:actions.delete')}
                </Button>
              </Col>
            </Row>
          ))}
          {labels.length === 0 && (
            <div style={{ textAlign: 'center', color: '#999', padding: '20px' }}>
              {t('config:create.noLabels')}
            </div>
          )}
        </Space>
      </Card>

      {/* Annotations */}
      <Card
        title={
          <Space>
            <span>{t('config:create.annotations')}</span>
            <Tooltip title={t('config:create.annotationsTooltip')}>
              <QuestionCircleOutlined style={{ color: '#999' }} />
            </Tooltip>
          </Space>
        }
        extra={
          <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={onAddAnnotation}>
            {t('config:create.addAnnotation')}
          </Button>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }} size="small">
          {annotations.map((annotation, index) => (
            <Row key={index} gutter={8} align="middle">
              <Col span={10}>
                <Input
                  placeholder={t('config:create.keyPlaceholder')}
                  value={annotation.key}
                  onChange={e => onAnnotationChange(index, 'key', e.target.value)}
                />
              </Col>
              <Col span={10}>
                <Input
                  placeholder={t('config:create.valuePlaceholder')}
                  value={annotation.value}
                  onChange={e => onAnnotationChange(index, 'value', e.target.value)}
                />
              </Col>
              <Col span={4}>
                <Button
                  type="text"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() => onRemoveAnnotation(index)}
                >
                  {t('common:actions.delete')}
                </Button>
              </Col>
            </Row>
          ))}
          {annotations.length === 0 && (
            <div style={{ textAlign: 'center', color: '#999', padding: '20px' }}>
              {t('config:create.noAnnotations')}
            </div>
          )}
        </Space>
      </Card>

      {/* Data content */}
      <Card
        title={
          <Space>
            <span>{t('config:create.dataContent')}</span>
            <Tooltip title={t('config:create.dataTooltip')}>
              <QuestionCircleOutlined style={{ color: '#999' }} />
            </Tooltip>
          </Space>
        }
        extra={
          <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={onAddDataItem}>
            {t('config:create.addDataItem')}
          </Button>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          {dataItems.map((item, index) => (
            <Card
              key={index}
              size="small"
              title={
                <Input
                  placeholder={t('config:create.dataKeyPlaceholder')}
                  value={item.key}
                  onChange={e => onDataKeyChange(index, e.target.value)}
                  style={{ width: '400px' }}
                />
              }
              extra={
                <Button
                  type="text"
                  danger
                  size="small"
                  icon={<DeleteOutlined />}
                  onClick={() => onRemoveDataItem(index)}
                >
                  {t('common:actions.delete')}
                </Button>
              }
            >
              <div style={{ border: '1px solid #d9d9d9', borderRadius: '4px' }}>
                <MonacoEditor
                  height="300px"
                  language="plaintext"
                  value={item.value}
                  onChange={value => onDataValueChange(index, value)}
                  options={{
                    minimap: { enabled: false },
                    lineNumbers: 'on',
                    scrollBeyondLastLine: false,
                    automaticLayout: true,
                    tabSize: 2,
                  }}
                  theme="vs-light"
                />
              </div>
            </Card>
          ))}
          {dataItems.length === 0 && (
            <div style={{ textAlign: 'center', color: '#999', padding: '20px' }}>
              {t('config:create.noDataItems')}
            </div>
          )}
        </Space>
      </Card>
    </>
  );
};

export default ConfigMapFormBody;
