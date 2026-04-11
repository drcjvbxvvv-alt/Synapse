// Log level colors for terminal display
export const levelColors: Record<string, string> = {
  error: '#ff4d4f',
  warn: '#faad14',
  info: '#1890ff',
  debug: '#8c8c8c',
};

// Log level Tag colors for Ant Design
export const levelTagColors: Record<string, string> = {
  error: 'red',
  warn: 'orange',
  info: 'blue',
  debug: 'default',
};

// Level options for filters
export const levelOptions = [
  { label: 'ERROR', value: 'error' },
  { label: 'WARN', value: 'warn' },
  { label: 'INFO', value: 'info' },
  { label: 'DEBUG', value: 'debug' },
];

// Event type options
export const eventTypeOptions = [
  { label: 'Normal', value: 'Normal' },
  { label: 'Warning', value: 'Warning' },
];
