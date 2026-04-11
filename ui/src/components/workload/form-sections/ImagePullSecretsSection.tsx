import React from 'react';
import { Form, Select, Card, Typography } from 'antd';
import type { ImagePullSecretsSectionProps } from './types';

const { Option } = Select;
const { Text } = Typography;

const ImagePullSecretsSection: React.FC<ImagePullSecretsSectionProps> = ({
  t,
  imagePullSecretsList,
}) => {
  return (
    <Card title={t('workloadForm.imagePullSecrets')} style={{ marginBottom: 16 }}>
      <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
        {t('workloadForm.imagePullSecretsDesc')}
      </Text>
      <Form.Item name="imagePullSecrets">
        <Select
          mode="multiple"
          placeholder={t('workloadForm.imagePullSecretsPlaceholder')}
          style={{ width: '100%' }}
          allowClear
        >
          {imagePullSecretsList.map((secret) => (
            <Option key={secret} value={secret}>
              {secret}
            </Option>
          ))}
        </Select>
      </Form.Item>
      {imagePullSecretsList.length === 0 && (
        <Text type="warning">
          {t('workloadForm.noDockerSecretWarning')}
        </Text>
      )}
    </Card>
  );
};

export default ImagePullSecretsSection;
