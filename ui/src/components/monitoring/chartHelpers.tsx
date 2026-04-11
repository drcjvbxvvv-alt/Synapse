import React from 'react';
import { Line, Area } from '@ant-design/plots';
import type { DataPoint, MultiSeriesDataPoint } from './types';

export const formatTimestamp = (timestamp: number) => {
  return new Date(timestamp * 1000).toLocaleTimeString();
};

export const formatValue = (value: number, unit: string = '') => {
  if (unit === '%') {
    return `${value.toFixed(2)}%`;
  }
  if (unit === 'bytes') {
    if (value >= 1024 * 1024 * 1024) {
      return `${(value / (1024 * 1024 * 1024)).toFixed(2)} GB`;
    } else if (value >= 1024 * 1024) {
      return `${(value / (1024 * 1024)).toFixed(2)} MB`;
    } else if (value >= 1024) {
      return `${(value / 1024).toFixed(2)} KB`;
    }
    return `${value.toFixed(2)} B`;
  }
  return value.toFixed(2);
};

export const convertBytesToUnit = (bytes: number): { value: number; unit: string } => {
  if (bytes >= 1024 * 1024 * 1024) {
    return { value: bytes / (1024 * 1024 * 1024), unit: 'GB' };
  } else if (bytes >= 1024 * 1024) {
    return { value: bytes / (1024 * 1024), unit: 'MB' };
  } else if (bytes >= 1024) {
    return { value: bytes / 1024, unit: 'KB' };
  }
  return { value: bytes, unit: 'B' };
};

export const renderChart = (data: DataPoint[], color: string, unit: string = '') => {
  const chartData = data.map(point => ({
    time: formatTimestamp(point.timestamp),
    value: point.value,
    timestamp: point.timestamp,
  }));

  const config = {
    data: chartData,
    xField: 'time',
    yField: 'value',
    height: 200,
    smooth: true,
    color: color,
    point: { size: 0 },
    tooltip: {
      formatter: (datum: { value: number; time: string }) => ({
        name: '數值',
        value: formatValue(datum.value, unit),
      }),
      title: (datum: { time: string }) => `時間: ${datum.time}`,
    },
    yAxis: {
      label: {
        formatter: (value: number) => formatValue(value, unit),
      },
    },
  };

  return <Line {...config} />;
};

export const renderMultiSeriesChart = (data: MultiSeriesDataPoint[], unit: string = '') => {
  if (!data || data.length === 0) {
    return <div style={{ textAlign: 'center', padding: '20px', color: '#999' }}>暫無資料</div>;
  }

  const chartData: Array<{ time: string; pod: string; value: number }> = [];
  data.forEach(point => {
    const time = formatTimestamp(point.timestamp);
    Object.entries(point.values).forEach(([podName, value]) => {
      if (value != null && typeof value === 'number' && !isNaN(value) && isFinite(value)) {
        chartData.push({ time, pod: podName, value });
      }
    });
  });

  if (chartData.length === 0) {
    return <div style={{ textAlign: 'center', padding: '20px', color: '#999' }}>暫無有效資料</div>;
  }

  const config = {
    data: chartData,
    xField: 'time',
    yField: 'value',
    colorField: 'pod',
    height: 300,
    smooth: true,
    point: { size: 0 },
    legend: {
      position: 'top' as const,
      maxRow: 3,
      layout: 'horizontal' as const,
    },
    yAxis: {
      label: {
        formatter: (value: string) => formatValue(parseFloat(value), unit),
      },
    },
  };

  return <Line {...config} />;
};

export const renderNetworkChart = (
  inData: DataPoint[],
  outData: DataPoint[],
  unit: string = '',
  inLabel: string = '入站',
  outLabel: string = '出站'
) => {
  let chartData;
  let yAxisSuffix = '';

  if (unit === 'bytes') {
    const maxValue = Math.max(
      ...inData.map(p => p.value),
      ...outData.map(p => p.value)
    );
    const { unit: bestUnit } = convertBytesToUnit(maxValue);
    yAxisSuffix = bestUnit;

    const divisor =
      bestUnit === 'GB' ? (1024 * 1024 * 1024) :
      bestUnit === 'MB' ? (1024 * 1024) :
      bestUnit === 'KB' ? 1024 : 1;

    chartData = inData.map((point, index) => ({
      time: formatTimestamp(point.timestamp),
      in: point.value / divisor,
      out: (outData[index]?.value || 0) / divisor,
      inRaw: point.value,
      outRaw: outData[index]?.value || 0,
      timestamp: point.timestamp,
    }));
  } else {
    chartData = inData.map((point, index) => ({
      time: formatTimestamp(point.timestamp),
      in: point.value,
      out: outData[index]?.value || 0,
      timestamp: point.timestamp,
    }));
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const config: any = {
    data: chartData,
    xField: 'time',
    yField: ['in', 'out'],
    height: 200,
    smooth: true,
    color: ['#1890ff', '#52c41a'],
    areaStyle: { fillOpacity: 0.6 },
    tooltip: {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      formatter: (datum: any) => {
        if (unit === 'bytes') {
          return [
            { name: inLabel, value: formatValue(datum.inRaw, 'bytes') },
            { name: outLabel, value: formatValue(datum.outRaw, 'bytes') },
          ];
        } else {
          return [
            { name: inLabel, value: datum.in.toFixed(2) },
            { name: outLabel, value: datum.out.toFixed(2) },
          ];
        }
      },
      title: (datum: { time: string }) => `時間: ${datum.time}`,
    },
    yAxis: {
      label: {
        formatter: (value: string) => {
          const numValue = parseFloat(value);
          return yAxisSuffix ? `${numValue.toFixed(2)} ${yAxisSuffix}` : numValue.toFixed(2);
        },
      },
    },
  };

  return <Area {...config} />;
};
