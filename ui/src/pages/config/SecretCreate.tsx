import React from 'react';
import {
  Card,
  Form,
  Input,
  Button,
  Space,
  Tag,
  Row,
  Col,
  Segmented,
  Select,
} from 'antd';
import {
  ArrowLeftOutlined,
  SaveOutlined,
  FormOutlined,
  CodeOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import MonacoEditor from '@monaco-editor/react';
import { useSecretCreate } from './hooks/useSecretCreate';
import KeyValueEditor from './components/KeyValueEditor';
import SecretDataEditor from './components/SecretDataEditor';

const SecretCreate: React.FC = () => {
  const navigate = useNavigate();
  const [form] = Form.useForm();

  const {
    clusterId,
    t,
    editMode,
    submitting,
    name, setName,
    namespace, setNamespace,
    secretType, setSecretType,
    serviceAccountName, setServiceAccountName,
    labels,
    annotations,
    dataItems,
    namespaces,
    loadingNamespaces,
    yamlContent, setYamlContent,
    loadNamespaces,
    handleAddLabel, handleRemoveLabel, handleLabelChange,
    handleAddAnnotation, handleRemoveAnnotation, handleAnnotationChange,
    handleAddDataItem, handleRemoveDataItem, handleDataKeyChange, handleDataValueChange, toggleDataVisibility,
    handleModeChange,
    handleSubmit,
  } = useSecretCreate();

  // Load namespaces on mount
  React.useEffect(() => { loadNamespaces(); }, [clusterId]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div style={{ padding: '24px' }}>
      <Space direction="vertical" style={{ width: '100%' }} size="large">

        {/* Header */}
        <Card>
          <Space style={{ width: '100%', justifyContent: 'space-between' }}>
            <Space>
              <Button
                icon={<ArrowLeftOutlined />}
                onClick={() => navigate(`/clusters/${clusterId}/configs`)}
              >
                {t('common:actions.back')}
              </Button>
              <h2 style={{ margin: 0 }}>{t('config:create.createSecret')}</h2>
              <Tag color="orange">{t('config:create.sensitiveData')}</Tag>
              <Segmented
                value={editMode}
                onChange={(value) => handleModeChange(value as 'form' | 'yaml')}
                options={[
                  { label: t('config:create.formMode'), value: 'form', icon: <FormOutlined /> },
                  { label: t('config:create.yamlMode'), value: 'yaml', icon: <CodeOutlined /> },
                ]}
              />
            </Space>
            <Space>
              <Button onClick={() => navigate(`/clusters/${clusterId}/configs`)}>
                {t('common:actions.cancel')}
              </Button>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                loading={submitting}
                onClick={handleSubmit}
              >
                {t('common:actions.create')}
              </Button>
            </Space>
          </Space>
        </Card>

        {/* YAML Editor */}
        {editMode === 'yaml' ? (
          <Card title={t('config:create.yamlEditor')} extra={<Tag color="orange">{t('config:create.yamlEditorSensitive')}</Tag>}>
            <div style={{ border: '1px solid #d9d9d9', borderRadius: '4px' }}>
              <MonacoEditor
                height="600px"
                language="yaml"
                value={yamlContent}
                onChange={(value) => setYamlContent(value || '')}
                options={{
                  minimap: { enabled: true },
                  fontSize: 14,
                  lineNumbers: 'on',
                  scrollBeyondLastLine: false,
                  automaticLayout: true,
                  tabSize: 2,
                  insertSpaces: true,
                  wordWrap: 'on',
                  folding: true,
                  bracketPairColorization: { enabled: true },
                }}
                theme="vs-light"
              />
            </div>
          </Card>
        ) : (
          <>
            {/* Basic Info */}
            <Card title={t('config:create.basicInfo')}>
              <Form form={form} layout="vertical">
                <Row gutter={16}>
                  <Col span={8}>
                    <Form.Item
                      label={t('config:create.name')}
                      required
                      help={t('config:create.secretNameHelp')}
                    >
                      <Input
                        placeholder={t('config:create.secretNamePlaceholder')}
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                      />
                    </Form.Item>
                  </Col>
                  <Col span={8}>
                    <Form.Item label={t('config:create.namespace')} help={t('config:create.secretNamespaceHelp')}>
                      <Select
                        value={namespace}
                        onChange={setNamespace}
                        placeholder={t('config:create.namespacePlaceholder')}
                        loading={loadingNamespaces}
                        showSearch
                        filterOption={(input, option) => {
                          if (!option?.children) return false;
                          return String(option.children).toLowerCase().includes(input.toLowerCase());
                        }}
                      >
                        {namespaces.map((ns) => (
                          <Select.Option key={ns} value={ns}>{ns}</Select.Option>
                        ))}
                      </Select>
                    </Form.Item>
                  </Col>
                  <Col span={8}>
                    <Form.Item label={t('config:create.type')} help={t('config:create.secretTypeHelp')}>
                      <Select value={secretType} onChange={setSecretType} placeholder={t('config:create.typePlaceholder')}>
                        <Select.Option value="Opaque">Opaque</Select.Option>
                        <Select.Option value="kubernetes.io/service-account-token">kubernetes.io/service-account-token</Select.Option>
                        <Select.Option value="kubernetes.io/dockerconfigjson">kubernetes.io/dockerconfigjson</Select.Option>
                        <Select.Option value="kubernetes.io/tls">kubernetes.io/tls</Select.Option>
                        <Select.Option value="kubernetes.io/basic-auth">kubernetes.io/basic-auth</Select.Option>
                        <Select.Option value="kubernetes.io/ssh-auth">kubernetes.io/ssh-auth</Select.Option>
                      </Select>
                    </Form.Item>
                  </Col>
                </Row>
                {secretType === 'kubernetes.io/service-account-token' && (
                  <Row gutter={16}>
                    <Col span={8}>
                      <Form.Item
                        label="ServiceAccount 名稱"
                        required
                        help="將自動加入 annotation: kubernetes.io/service-account.name"
                      >
                        <Input
                          placeholder="請輸入 ServiceAccount 名稱"
                          value={serviceAccountName}
                          onChange={(e) => setServiceAccountName(e.target.value)}
                        />
                      </Form.Item>
                    </Col>
                  </Row>
                )}
              </Form>
            </Card>

            {/* Labels */}
            <KeyValueEditor
              title={t('config:create.labels')}
              tooltip={t('config:create.labelsTooltip')}
              addLabel={t('config:create.addLabel')}
              emptyText={t('config:create.noLabels')}
              items={labels}
              onAdd={handleAddLabel}
              onRemove={handleRemoveLabel}
              onChange={handleLabelChange}
            />

            {/* Annotations */}
            <KeyValueEditor
              title={t('config:create.annotations')}
              tooltip={t('config:create.annotationsTooltip')}
              addLabel={t('config:create.addAnnotation')}
              emptyText={t('config:create.noAnnotations')}
              items={annotations}
              onAdd={handleAddAnnotation}
              onRemove={handleRemoveAnnotation}
              onChange={handleAnnotationChange}
            />

            {/* Data Content */}
            <SecretDataEditor
              items={dataItems}
              onAdd={handleAddDataItem}
              onRemove={handleRemoveDataItem}
              onKeyChange={handleDataKeyChange}
              onValueChange={handleDataValueChange}
              onToggleVisibility={toggleDataVisibility}
            />
          </>
        )}
      </Space>
    </div>
  );
};

export default SecretCreate;
