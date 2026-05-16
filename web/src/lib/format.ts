import type { SessionStatus } from "@/types";

export function humanStatus(status: SessionStatus | undefined): string {
  switch (status) {
    case "pending":
      return "待开始";
    case "processing":
      return "润色进行中";
    case "paused":
      return "已暂停";
    case "completed":
      return "已完成";
    case "failed":
      return "需要处理";
    default:
      return "就绪";
  }
}

export type StatusTone =
  | "default"
  | "processing"
  | "success"
  | "warning"
  | "error";

export function statusTone(status: SessionStatus | undefined): StatusTone {
  switch (status) {
    case "processing":
      return "processing";
    case "paused":
      return "warning";
    case "completed":
      return "success";
    case "failed":
      return "error";
    default:
      return "default";
  }
}

export function statusDescription(
  status: SessionStatus | undefined,
  completedPasses: number,
  totalPasses: number,
): string {
  switch (status) {
    case "pending":
      return completedPasses === 0
        ? "文档已上传，等待开始第一轮润色。"
        : `已完成第 ${completedPasses} 轮。${
            completedPasses < totalPasses
              ? "如需继续降低模板化风险，可选择开始下一轮；也可以直接导出当前稿。"
              : "文档已全部完成。"
          }`;
    case "processing":
      return "正在逐段润色文档。";
    case "paused":
      return "处理已暂停，随时可以继续。";
    case "completed":
      return "全部轮次已完成,最新稿可下载。";
    case "failed":
      return "处理中止,请查看活动记录后重试。";
    default:
      return "上传文档以开始。";
  }
}

export function formatCharDelta(charDelta: number): string {
  if (charDelta > 0) return `字数变化 +${charDelta}`;
  if (charDelta < 0) return `字数变化 ${charDelta}`;
  return "字数基本未变";
}

export function formatDate(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(new Date(value));
}

export function formatClock(value: number): string {
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "numeric",
    minute: "2-digit",
  }).format(new Date(value));
}

export function toErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return "请求未能完成,请稍后再试。";
}

export function formatBytes(bytes: number): string {
  if (!bytes) return "—";
  const kb = bytes / 1024;
  if (kb < 1024) return `${kb.toFixed(1)} KB`;
  return `${(kb / 1024).toFixed(2)} MB`;
}

export function formatRelativeTime(value: string | undefined | null): string {
  if (!value) return "—";
  const parsed = new Date(value);
  const ms = parsed.getTime();
  if (!Number.isFinite(ms)) return "—";

  const diff = Date.now() - ms;
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;
  const week = 7 * day;

  if (diff < 30_000 && diff > -30_000) return "刚刚";
  if (diff >= 0) {
    if (diff < hour) return `${Math.floor(diff / minute)} 分钟前`;
    if (diff < day) return `${Math.floor(diff / hour)} 小时前`;
    if (diff < week) return `${Math.floor(diff / day)} 天前`;
  } else {
    const ahead = -diff;
    if (ahead < hour) return `${Math.floor(ahead / minute)} 分钟后`;
    if (ahead < day) return `${Math.floor(ahead / hour)} 小时后`;
  }
  return formatDate(value);
}

export function formatDuration(seconds: number | undefined | null): string {
  if (!seconds || seconds <= 0) return "—";
  if (seconds < 60) return `${Math.round(seconds)} 秒`;
  const minutes = Math.floor(seconds / 60);
  const remainder = Math.round(seconds % 60);
  if (minutes < 60)
    return remainder ? `${minutes} 分 ${remainder} 秒` : `${minutes} 分`;
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return remainingMinutes
    ? `${hours} 小时 ${remainingMinutes} 分`
    : `${hours} 小时`;
}

export function promptProfileLabel(profile: string | undefined): string {
  switch (profile) {
    case "cn":
      return "中文润色";
    case "cn_single":
      return "中文(仅一轮)润色";
    case "en":
      return "英文润色";
    default:
      return profile ?? "—";
  }
}
