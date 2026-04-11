export const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

export const formatCPU = (cores: number): string => {
  if (cores < 1) {
    return (cores * 1000).toFixed(0) + 'm';
  }
  return cores.toFixed(2) + ' cores';
};

export const formatTime = (timestamp: number): string => {
  return new Date(timestamp * 1000).toLocaleString('zh-TW');
};

export const getHealthColor = (status: string): string => {
  switch (status) {
    case 'healthy': return '#52c41a';
    case 'warning': return '#faad14';
    case 'critical': return '#ff4d4f';
    default: return '#d9d9d9';
  }
};
