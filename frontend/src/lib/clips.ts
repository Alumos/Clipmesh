import type { Clip } from "@/types"

export type ClipFilter = "all" | "text" | "file"

export function filterClips(clips: Clip[], search: string, filter: ClipFilter) {
  const query = search.trim().toLowerCase()
  return clips.filter((clip) => {
    if (filter !== "all" && clip.kind !== filter) return false
    if (!query) return true
    const content = clip.kind === "text" ? clipPlainText(clip) : `${clip.name ?? ""} ${clip.mimeType ?? ""}`
    return `${content} ${clip.deviceName}`.toLowerCase().includes(query)
  })
}

export function clipPlainText(clip: Clip) {
  return clip.formats?.["text/plain"] ?? clip.preview ?? ""
}

export function isImageClip(clip: Clip) {
  if (clip.kind !== "file") return false
  if (clip.mimeType?.startsWith("image/")) return true
  return /\.(avif|bmp|gif|heic|heif|jpe?g|png|svg|webp)$/i.test(clip.name ?? "")
}

export function formatClipExpiry(value: string) {
  const expiresAt = new Date(value).toLocaleString("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
  return `保留至 ${expiresAt}`
}
