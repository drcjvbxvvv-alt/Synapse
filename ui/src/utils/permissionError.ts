/**
 * showPermissionDenied — 統一的「無權限操作」通知。
 *
 * 使用靜態 notification API，可在 React 元件外部呼叫（如 axios 攔截器）。
 * notification.key = 'permission-denied' 確保同一時間只顯示一則。
 */

import { notification } from 'antd';
import i18n from '../i18n';

const NOTIFICATION_KEY = 'permission-denied';

/**
 * @param feature 可選的功能鍵，用於顯示更具體的說明（如 'terminal:pod'）
 */
export function showPermissionDenied(feature?: string): void {
  const description = feature
    ? i18n.t('common:messages.permissionDeniedFeature', { feature })
    : i18n.t('common:messages.permissionDeniedDesc');

  notification.warning({
    key: NOTIFICATION_KEY,
    message: i18n.t('common:messages.permissionDenied'),
    description,
    duration: 4,
    placement: 'topRight',
  });
}
