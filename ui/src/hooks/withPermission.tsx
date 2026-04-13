/**
 * 權限檢查高階元件
 * 用於包裝需要權限檢查的元件
 */

import React from 'react';
import { useTranslation } from 'react-i18next';
import ErrorPage from '../components/ErrorPage';
import { usePermission } from './usePermission';

export const withPermission = <P extends object>(
  WrappedComponent: React.ComponentType<P>,
  requiredAction?: string
) => {
  return (props: P) => {
    const { canPerformAction } = usePermission();
    const { t } = useTranslation('components');

    if (requiredAction && !canPerformAction(requiredAction)) {
      return (
        <ErrorPage
          status={403}
          title={t('permissionGuard.noAccess')}
          subTitle={t('permissionGuard.platformAdminOnly')}
          showHome
          showBack
        />
      );
    }

    return <WrappedComponent {...props} />;
  };
};
