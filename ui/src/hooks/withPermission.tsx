/**
 * 權限檢查高階元件
 * 用於包裝需要權限檢查的元件
 */

import React from 'react';
import ErrorPage from '../components/ErrorPage';
import { usePermission } from './usePermission';

export const withPermission = <P extends object>(
  WrappedComponent: React.ComponentType<P>,
  requiredAction?: string
) => {
  return (props: P) => {
    const { canPerformAction } = usePermission();

    if (requiredAction && !canPerformAction(requiredAction)) {
      return (
        <ErrorPage
          status={403}
          title="無權限"
          subTitle="您沒有權限訪問此內容，請聯絡管理員申請授權。"
          showHome
          showBack
        />
      );
    }

    return <WrappedComponent {...props} />;
  };
};
