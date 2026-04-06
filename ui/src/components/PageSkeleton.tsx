/**
 * PageSkeleton — 頁面載入骨架屏
 *
 * 取代各頁面散落的 `<Spin size="large" />` 裸露載入。
 *
 * variants:
 *   - table  : 工具列 + 表格列（列表頁預設）
 *   - detail : 標題 + 表單欄位（詳情 / 設定頁預設）
 *   - cards  : 統計卡片格線（Dashboard 類頁面）
 */
import React from 'react';
import { Skeleton, Card, Space } from 'antd';

interface PageSkeletonProps {
  variant?: 'table' | 'detail' | 'cards';
  /** table variant 的骨架列數，預設 6 */
  rows?: number;
  /** 是否顯示外層 padding，預設 true */
  padded?: boolean;
}

const TableSkeleton: React.FC<{ rows: number }> = ({ rows }) => (
  <div>
    {/* 工具列：標題 + 按鈕 */}
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
      <Skeleton.Input active style={{ width: 160, height: 28 }} />
      <Space>
        <Skeleton.Button active style={{ width: 90 }} />
        <Skeleton.Button active style={{ width: 80 }} />
      </Space>
    </div>
    {/* 搜尋 / 篩選列 */}
    <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
      <Skeleton.Input active style={{ width: 220, height: 32 }} />
      <Skeleton.Button active style={{ width: 100, height: 32 }} />
    </div>
    {/* 表格主體 */}
    <Card styles={{ body: { padding: 0 } }}>
      {/* 表頭 */}
      <div style={{
        display: 'flex', gap: 12, padding: '12px 16px',
        borderBottom: '1px solid #f0f0f0', background: '#f8f9fa',
      }}>
        {[22, 14, 10, 12, 16].map((w, i) => (
          <Skeleton.Input key={i} active style={{ width: `${w}%`, height: 16 }} />
        ))}
      </div>
      {/* 表格列 */}
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} style={{
          display: 'flex', gap: 12, padding: '14px 16px',
          borderBottom: i < rows - 1 ? '1px solid #f5f5f5' : 'none',
          alignItems: 'center',
        }}>
          {[22, 14, 10, 12, 16].map((w, j) => (
            <Skeleton.Input key={j} active style={{ width: `${w}%`, height: 14 }} />
          ))}
        </div>
      ))}
    </Card>
  </div>
);

const DetailSkeleton: React.FC = () => (
  <div>
    {/* 頁面標題區 */}
    <div style={{ marginBottom: 24 }}>
      <Skeleton.Input active style={{ width: 200, height: 28, marginBottom: 8, display: 'block' }} />
      <Skeleton.Input active style={{ width: 320, height: 16 }} />
    </div>
    <Card>
      {/* 表單欄位 */}
      {Array.from({ length: 5 }).map((_, i) => (
        <div key={i} style={{ marginBottom: 20 }}>
          <Skeleton.Input active style={{ width: 80, height: 14, marginBottom: 8, display: 'block' }} />
          <Skeleton.Input active style={{ width: '100%', height: 32 }} />
        </div>
      ))}
      {/* 操作按鈕 */}
      <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
        <Skeleton.Button active style={{ width: 88 }} />
        <Skeleton.Button active style={{ width: 72 }} />
      </div>
    </Card>
  </div>
);

const CardsSkeleton: React.FC = () => (
  <div>
    {/* 頁面標題 */}
    <Skeleton.Input active style={{ width: 180, height: 28, marginBottom: 20, display: 'block' }} />
    {/* 統計卡片列 */}
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 16, marginBottom: 20 }}>
      {Array.from({ length: 4 }).map((_, i) => (
        <Card key={i}>
          <Skeleton.Input active style={{ width: '60%', height: 14, marginBottom: 8, display: 'block' }} />
          <Skeleton.Input active style={{ width: '40%', height: 28 }} />
        </Card>
      ))}
    </div>
    {/* 內容卡片 */}
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
      {Array.from({ length: 2 }).map((_, i) => (
        <Card key={i}>
          <Skeleton active paragraph={{ rows: 4 }} />
        </Card>
      ))}
    </div>
  </div>
);

const PageSkeleton: React.FC<PageSkeletonProps> = ({
  variant = 'table',
  rows = 6,
  padded = true,
}) => {
  const content = (() => {
    switch (variant) {
      case 'detail': return <DetailSkeleton />;
      case 'cards':  return <CardsSkeleton />;
      default:       return <TableSkeleton rows={rows} />;
    }
  })();

  return (
    <div style={padded ? { padding: '0 0 24px' } : undefined}>
      {content}
    </div>
  );
};

export default PageSkeleton;
