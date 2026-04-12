import React from "react";
import { Tag, Badge } from "antd";

const STATUS_CONFIG: Record<string, { color: string; text?: string }> = {
  // Pod / Deployment
  Running: { color: "success" },
  Succeeded: { color: "success" },
  Pending: { color: "processing" },
  ContainerCreating: { color: "processing" },
  Terminating: { color: "error", text: "終止中" },
  Failed: { color: "error" },
  CrashLoopBackOff: { color: "error" },
  ImagePullBackOff: { color: "error" },
  OOMKilled: { color: "error" },
  Unknown: { color: "default" },

  // Node
  Ready: { color: "success" },
  NotReady: { color: "error" },

  // 通用
  Active: { color: "success", text: "執行中" },
  Inactive: { color: "default" },
  enabled: { color: "success", text: "啟用" },
  disabled: { color: "default", text: "停用" },

  // Gateway API
  Accepted: { color: "success" },
  Programmed: { color: "success" },
  NotAccepted: { color: "error" },

  // StorageClass ReclaimPolicy
  Delete: { color: "error" },
  Retain: { color: "warning" },
  Recycle: { color: "default" },

  // StorageClass VolumeBindingMode
  Immediate: { color: "processing" },
  WaitForFirstConsumer: { color: "default", text: "WFC" },

  // 通用布林值
  true_yes: { color: "success", text: "是" },
  false_no: { color: "default", text: "否" },
};

interface StatusTagProps {
  status: string;
  showDot?: boolean; // 用 Badge dot 模式
}

export const StatusTag = React.memo(function StatusTag({ status, showDot = false }: StatusTagProps) {
  const config = STATUS_CONFIG[status] ?? { color: "default" };
  const label = config.text ?? status;

  if (showDot) {
    const statusMap: Record<
      string,
      "success" | "processing" | "error" | "warning" | "default"
    > = {
      success: "success",
      processing: "processing",
      error: "error",
      warning: "warning",
    };
    return <Badge status={statusMap[config.color] ?? "default"} text={label} />;
  }

  return <Tag color={config.color}>{label}</Tag>;
});
