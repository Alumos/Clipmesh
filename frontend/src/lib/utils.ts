import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatBytes(bytes = 0) {
  if (bytes < 1024) return `${bytes} B`
  const units = ["KB", "MB", "GB", "TB"]
  let value = bytes
  let unit = -1
  do {
    value /= 1024
    unit += 1
  } while (value >= 1024 && unit < units.length - 1)
  return `${value.toFixed(value >= 10 ? 0 : 1)} ${units[unit]}`
}

export function formatRelativeTime(dateString: string) {
  const date = new Date(dateString)
  const delta = Math.max(0, Date.now() - date.getTime())
  const minutes = Math.floor(delta / 60_000)
  if (minutes < 1) return "刚刚"
  if (minutes < 60) return `${minutes} 分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时前`
  const days = Math.floor(hours / 24)
  if (days < 7) return `${days} 天前`
  return date.toLocaleString("zh-CN", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })
}

export function formatDuration(seconds: number) {
  if (seconds < 60 * 60) return `${Math.max(1, Math.round(seconds / 60))} 分钟`
  if (seconds < 24 * 60 * 60) return `${Math.round(seconds / 3600)} 小时`
  return `${Math.round(seconds / (24 * 3600))} 天`
}
