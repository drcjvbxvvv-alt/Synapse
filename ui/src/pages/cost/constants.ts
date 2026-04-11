// Chart color palette (AntV G2 Colorful)
// chart-only: data visualization colors are exempt from design token rule
export const COLORS = [
  '#5B8FF9', '#5AD8A6', '#F6BD16', '#E8684A',
  '#6DC8EC', '#9867BC', '#FF9D4D', '#269A99',
];

export const BAR_PROPS = {
  radius: [5, 5, 0, 0] as [number, number, number, number],
  maxBarSize: 44,
  isAnimationActive: true,
  animationBegin: 0,
  animationDuration: 800,
  animationEasing: 'ease-out' as const,
};

export const TOOLTIP_STYLE = {
  contentStyle: {
    borderRadius: 10,
    border: 'none',
    boxShadow: '0 6px 24px rgba(0,0,0,0.10)',
    fontSize: 13,
  },
};

export const GRID_STYLE = { stroke: '#f0f0f0', strokeDasharray: '4 4' };
