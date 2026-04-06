/**
 * 命名空間選擇器元件
 * 根據使用者權限自動過濾可選的命名空間
 */

import React, { useEffect, useState, useMemo } from 'react';
import { Select, Tag, Tooltip } from 'antd';
import { LockOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { usePermission } from '../hooks/usePermission';
import { namespaceService } from '../services/namespaceService';

const { Option } = Select;

interface NamespaceSelectorProps {
  clusterId: string | number;
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
  allowAll?: boolean; // 是否允許選擇"全部命名空間"
  style?: React.CSSProperties;
  disabled?: boolean;
  showPermissionHint?: boolean; // 是否顯示權限提示
}

const NamespaceSelector: React.FC<NamespaceSelectorProps> = ({
  clusterId,
  value,
  onChange,
  placeholder,
  allowAll = true,
  style,
  disabled = false,
  showPermissionHint = true,
}) => {
  const { t } = useTranslation('components');
  const [allNamespaces, setAllNamespaces] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const { filterNamespaces, hasAllNamespaceAccess, getAllowedNamespaces } = usePermission();

  // 載入命名空間列表
  useEffect(() => {
    const fetchNamespaces = async () => {
      if (!clusterId) return;
      
      setLoading(true);
      try {
        const response = await namespaceService.getNamespaces(String(clusterId));
        if (response) {
          const names = (response as ({ name?: string } | string)[]).map((ns: { name?: string } | string) => 
            typeof ns === 'string' ? ns : (ns.name || '')
          );
          setAllNamespaces(names.filter(Boolean));
        }
      } catch (error) {
        console.error('Failed to fetch namespaces:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchNamespaces();
  }, [clusterId]);

  // 根據權限過濾命名空間
  const filteredNamespaces = useMemo(() => {
    return filterNamespaces(allNamespaces, clusterId);
  }, [allNamespaces, filterNamespaces, clusterId]);

  // 檢查是否有全部命名空間權限
  const hasFullAccess = hasAllNamespaceAccess(clusterId);
  
  // 獲取允許的命名空間配置（用於顯示提示）
  const allowedConfig = getAllowedNamespaces(clusterId);

  // 權限提示文案
  const getPermissionHint = () => {
    if (hasFullAccess) {
      return t('namespaceSelector.fullAccessHint');
    }
    if (allowedConfig.length === 0) {
      return t('namespaceSelector.noAccessHint');
    }
    return t('namespaceSelector.limitedAccessHint', { namespaces: allowedConfig.join(', ') });
  };

  return (
    <div style={{ display: 'inline-flex', alignItems: 'center', gap: 8, ...style }}>
      <Select
        value={value}
        onChange={onChange}
        placeholder={placeholder || t('namespaceSelector.placeholder')}
        loading={loading}
        disabled={disabled}
        style={{ minWidth: 180 }}
        allowClear={allowAll}
        showSearch
        filterOption={(input, option) =>
          (option?.children as unknown as string)?.toLowerCase().includes(input.toLowerCase())
        }
      >
        {allowAll && hasFullAccess && (
          <Option value="">{t('namespaceSelector.allNamespaces')}</Option>
        )}
        {filteredNamespaces.map((ns) => (
          <Option key={ns} value={ns}>
            {ns}
          </Option>
        ))}
      </Select>
      
      {showPermissionHint && !hasFullAccess && (
        <Tooltip title={getPermissionHint()}>
          <Tag icon={<LockOutlined />} color="warning" style={{ margin: 0 }}>
            {t('namespaceSelector.restricted')}
          </Tag>
        </Tooltip>
      )}
    </div>
  );
};

export default React.memo(NamespaceSelector);
