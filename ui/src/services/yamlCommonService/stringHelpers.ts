// ==================== 輔助函式 ====================

export const parseCommaString = (str: string | undefined): string[] => {
  if (!str) return [];
  return str.split(',').map(s => s.trim()).filter(s => s);
};

export const parseCommandString = (str: string | string[] | undefined): string[] => {
  if (!str) return [];
  if (Array.isArray(str)) return str;
  return str.split('\n').map(s => s.trim()).filter(s => s);
};

export const commandArrayToString = (arr: string[] | undefined): string => {
  if (!arr || arr.length === 0) return '';
  return arr.join('\n');
};
