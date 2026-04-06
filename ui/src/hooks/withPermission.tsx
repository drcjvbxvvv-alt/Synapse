/**
 * 權限檢查高階元件
 * 用於包裝需要權限檢查的元件
 */

import React from 'react';
import { Result } from 'antd';
import { usePermission } from './usePermission';

export const withPermission = <P extends object>(
  WrappedComponent: React.ComponentType<P>,
  requiredAction?: string
) => {
  return (props: P) => {
    const { canPerformAction } = usePermission();
    
    if (requiredAction && !canPerformAction(requiredAction)) {
      return <Result status="403" title="無權限" subTitle="您沒有權限訪問此內容" />;
    }
    
    return <WrappedComponent {...props} />;
  };
};

