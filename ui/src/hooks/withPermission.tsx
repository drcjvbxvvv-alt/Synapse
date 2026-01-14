/**
 * 权限检查高阶组件
 * 用于包装需要权限检查的组件
 */

import React from 'react';
import { usePermission } from './usePermission';

export const withPermission = <P extends object>(
  WrappedComponent: React.ComponentType<P>,
  requiredAction?: string
) => {
  return (props: P) => {
    const { canPerformAction } = usePermission();
    
    if (requiredAction && !canPerformAction(requiredAction)) {
      return null; // 或者返回无权限提示组件
    }
    
    return <WrappedComponent {...props} />;
  };
};

