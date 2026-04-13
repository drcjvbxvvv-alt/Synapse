import React from 'react';
import { Tag } from 'antd';

// Use antd preset tag color names (theme-aware, no hardcoded hex)
export const SEVERITY_COLORS: Record<string, string> = {
  CRITICAL: 'red',
  HIGH: 'orange',
  MEDIUM: 'gold',
  LOW: 'green',
  UNKNOWN: 'default',
};

interface SeverityTagProps {
  severity: string;
  count: number;
}

export function SeverityTag({ severity, count }: SeverityTagProps) {
  if (count === 0) return null;
  return (
    <Tag color={SEVERITY_COLORS[severity] ?? 'default'} style={{ fontWeight: 600 }}>
      {severity[0]}: {count}
    </Tag>
  );
}
