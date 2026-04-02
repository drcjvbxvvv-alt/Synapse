import React, { useState } from 'react';
import { Alert, Input, Modal, Space, Typography } from 'antd';
import { LockOutlined, WarningOutlined } from '@ant-design/icons';

const { Text } = Typography;

interface ProtectedConfirmProps {
  open: boolean;
  resourceName: string;
  namespace: string;
  action: string; // '刪除' / '縮容至 0' / '更新'
  onConfirm: () => void;
  onCancel: () => void;
  /** 審批模式：不需輸入名稱，而是建立審批請求 */
  approvalMode?: boolean;
  onRequestApproval?: () => void;
}

/**
 * ProtectedConfirm：受保護命名空間的危險操作確認
 * - 一般模式：要求使用者輸入資源名稱確認
 * - 審批模式：改為提交審批請求
 */
const ProtectedConfirm: React.FC<ProtectedConfirmProps> = ({
  open,
  resourceName,
  namespace,
  action,
  onConfirm,
  onCancel,
  approvalMode = false,
  onRequestApproval,
}) => {
  const [inputValue, setInputValue] = useState('');
  const matches = inputValue === resourceName;

  const handleOk = () => {
    if (approvalMode) {
      onRequestApproval?.();
    } else if (matches) {
      onConfirm();
    }
  };

  return (
    <Modal
      open={open}
      title={
        <Space>
          <LockOutlined style={{ color: '#fa8c16' }} />
          受保護命名空間 — {approvalMode ? '需要審批' : '危險操作確認'}
        </Space>
      }
      onOk={handleOk}
      onCancel={onCancel}
      okText={approvalMode ? '送出審批請求' : action}
      okButtonProps={{
        danger: !approvalMode,
        disabled: !approvalMode && !matches,
      }}
      cancelText="取消"
    >
      <Alert
        icon={<WarningOutlined />}
        message={`命名空間「${namespace}」為受保護環境`}
        description={
          approvalMode
            ? `${action}操作需要管理員核准。送出後請等待審批人批准。`
            : `您即將在生產環境執行「${action}」操作，此操作不可輕易撤銷。`
        }
        type={approvalMode ? 'warning' : 'error'}
        showIcon
        style={{ marginBottom: 16 }}
      />

      {!approvalMode && (
        <>
          <Text>請輸入資源名稱 <Text code>{resourceName}</Text> 以確認操作：</Text>
          <Input
            style={{ marginTop: 8 }}
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            placeholder={`輸入 ${resourceName} 確認`}
            status={inputValue && !matches ? 'error' : undefined}
            autoFocus
          />
          {inputValue && !matches && (
            <Text type="danger" style={{ fontSize: 12 }}>名稱不符，請重新輸入</Text>
          )}
        </>
      )}
    </Modal>
  );
};

export default ProtectedConfirm;
