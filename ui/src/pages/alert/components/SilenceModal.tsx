import React from 'react';
import {
  Modal,
  Form,
  DatePicker,
  Input,
  Button,
  Space,
  Select,
  Alert as AntAlert,
} from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import type { FormInstance } from 'antd';
import type { TFunction } from 'i18next';

const { RangePicker } = DatePicker;
const { Option } = Select;

interface SilenceModalProps {
  open: boolean;
  form: FormInstance;
  t: TFunction;
  onOk: () => void;
  onCancel: () => void;
}

const SilenceModal: React.FC<SilenceModalProps> = ({ open, form, t, onOk, onCancel }) => (
  <Modal
    title={t('alert:center.createSilenceTitle')}
    open={open}
    onOk={onOk}
    onCancel={onCancel}
    width={600}
    okText={t('alert:center.createBtn')}
    cancelText={t('common:actions.cancel')}
  >
    <Form form={form} layout="vertical">
      <Form.Item
        label={t('alert:center.effectiveTimeRange')}
        name="timeRange"
        rules={[{ required: true, message: t('alert:center.effectiveTimeRequired') }]}
      >
        <RangePicker showTime format="YYYY-MM-DD HH:mm" style={{ width: '100%' }} />
      </Form.Item>

      <Form.Item label={t('alert:center.matchRulesLabel')} required>
        <AntAlert
          message={t('alert:center.matchRulesLabel')}
          description={t('alert:center.matchRulesDesc')}
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
        <Form.List name="matchers">
          {(fields, { add, remove }) => (
            <>
              {fields.map(({ key, name, ...restField }) => (
                <Space key={key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                  <Form.Item
                    {...restField}
                    name={[name, 'name']}
                    rules={[{ required: true, message: t('alert:center.labelNameRequired') }]}
                  >
                    <Input placeholder={t('alert:center.labelName')} style={{ width: 120 }} />
                  </Form.Item>
                  <Form.Item {...restField} name={[name, 'isEqual']} initialValue={true}>
                    <Select style={{ width: 80 }}>
                      <Option value={true}>=</Option>
                      <Option value={false}>!=</Option>
                    </Select>
                  </Form.Item>
                  <Form.Item
                    {...restField}
                    name={[name, 'value']}
                    rules={[{ required: true, message: t('alert:center.valueRequired') }]}
                  >
                    <Input placeholder={t('alert:center.value')} style={{ width: 150 }} />
                  </Form.Item>
                  <Form.Item {...restField} name={[name, 'isRegex']} valuePropName="checked">
                    <Select style={{ width: 80 }} defaultValue={false}>
                      <Option value={false}>{t('alert:center.exact')}</Option>
                      <Option value={true}>{t('alert:center.regex')}</Option>
                    </Select>
                  </Form.Item>
                  <Button
                    type="link"
                    danger
                    onClick={() => remove(name)}
                    icon={<DeleteOutlined />}
                  />
                </Space>
              ))}
              <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                {t('alert:center.addMatchRule')}
              </Button>
            </>
          )}
        </Form.List>
      </Form.Item>

      <Form.Item
        label={t('alert:center.remarkLabel')}
        name="comment"
        rules={[{ required: true, message: t('alert:center.remarkRequired') }]}
      >
        <Input.TextArea rows={3} placeholder={t('alert:center.remarkPlaceholder')} />
      </Form.Item>
    </Form>
  </Modal>
);

export default SilenceModal;
