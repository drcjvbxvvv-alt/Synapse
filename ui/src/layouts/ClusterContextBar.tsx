import React from 'react';
import ClusterSelector from '../components/ClusterSelector';

/**
 * 叢集範疇頁面的頂部 Context Bar。
 * 僅在 isClusterDetail 為 true 時由 MainLayout 渲染。
 * 顯示叢集切換器，幫助使用者明確知道目前操作的是哪個叢集。
 */
const ClusterContextBar: React.FC = () => (
  <div
    style={{
      position: 'fixed',
      top: 48,
      left: 0,
      right: 0,
      width: '100%',
      height: 48,
      background: '#f8fafc',
      borderBottom: '1px solid #f0f0f0',
      zIndex: 999,
      display: 'flex',
      alignItems: 'center',
      padding: '0 24px',
    }}
  >
    <ClusterSelector />
  </div>
);

export default React.memo(ClusterContextBar);
