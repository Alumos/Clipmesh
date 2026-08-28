import { useEffect, useState } from "react"
import {
  Clipboard,
  Copy,
  Download,
  FileUp,
  Image as ImageIcon,
  Loader2,
  Trash2,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { downloadFile } from "@/lib/api"
import { clipPlainText, formatClipExpiry, isImageClip } from "@/lib/clips"
import { formatBytes, formatRelativeTime } from "@/lib/utils"
import type { Clip } from "@/types"

interface ClipRowProps {
  clip: Clip
  copying: boolean
  deleting: boolean
  onCopy: () => void
  onDownload: () => void
  onDelete: () => void
}

export function ClipRow({
  clip,
  copying,
  deleting,
  onCopy,
  onDownload,
  onDelete,
}: ClipRowProps) {
  const image = isImageClip(clip)
  const hasHTML = Boolean(clip.formats?.["text/html"])
  const primaryAction = clip.kind === "text" ? onCopy : onDownload

  return (
    <article className="group flex items-start gap-3 bg-background px-4 py-3.5 transition-colors hover:bg-muted/30 sm:px-5 sm:py-4">
      {image ? (
        <ImagePreview clip={clip} />
      ) : (
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border bg-muted/55 text-muted-foreground">
          {clip.kind === "text" ? (
            <Clipboard className="h-4 w-4" />
          ) : (
            <FileUp className="h-4 w-4" />
          )}
        </div>
      )}

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1">
          <Badge
            variant="outline"
            className="h-5 rounded-md px-1.5 text-[10px] font-medium"
          >
            {clip.kind === "text" ? "文本" : image ? "图片" : "文件"}
          </Badge>
          {hasHTML && (
            <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
              富文本
            </span>
          )}
          <span className="max-w-36 truncate text-xs text-muted-foreground sm:max-w-52">
            {clip.deviceName || "未知设备"}
          </span>
          <span className="text-xs text-border">·</span>
          <time
            dateTime={clip.createdAt}
            className="shrink-0 text-xs text-muted-foreground"
          >
            {formatRelativeTime(clip.createdAt)}
          </time>
        </div>

        {clip.kind === "text" ? (
          <p className="clip-text-preview mt-2 whitespace-pre-wrap break-words text-sm leading-5 text-foreground/90">
            {clipPlainText(clip) || "空白文本"}
          </p>
        ) : (
          <div className="mt-2 min-w-0">
            <p className="truncate text-sm font-medium">{clip.name || "未命名文件"}</p>
            <p className="mt-1 truncate text-xs text-muted-foreground">
              {clip.mimeType || "未知格式"} · {formatBytes(clip.size)}
              {clip.expiresAt && ` · ${formatClipExpiry(clip.expiresAt)}`}
            </p>
          </div>
        )}
      </div>

      <div className="flex shrink-0 flex-col gap-1 sm:flex-row">
        <Button
          variant="secondary"
          size="icon"
          className="h-8 w-8"
          onClick={primaryAction}
          disabled={copying}
          aria-label={clip.kind === "text" ? "复制文本" : "下载文件"}
        >
          {copying ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : clip.kind === "text" ? (
            <Copy className="h-3.5 w-3.5" />
          ) : (
            <Download className="h-3.5 w-3.5" />
          )}
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 text-muted-foreground hover:bg-muted hover:text-foreground"
          onClick={onDelete}
          disabled={deleting}
          aria-label="删除记录"
        >
          {deleting ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <Trash2 className="h-3.5 w-3.5" />
          )}
        </Button>
      </div>
    </article>
  )
}

function ImagePreview({ clip }: { clip: Clip }) {
  const [source, setSource] = useState("")
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    let active = true
    let objectUrl = ""
    setSource("")
    setFailed(false)

    void downloadFile(clip.id)
      .then((blob) => {
        if (!active) return
        objectUrl = URL.createObjectURL(blob)
        setSource(objectUrl)
      })
      .catch(() => {
        if (active) setFailed(true)
      })

    return () => {
      active = false
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [clip.id])

  return (
    <Dialog>
      <DialogTrigger asChild>
        <button
          type="button"
          disabled={!source}
          className="relative flex h-16 w-20 shrink-0 items-center justify-center overflow-hidden rounded-lg border bg-muted sm:h-[4.5rem] sm:w-24"
          aria-label={`预览 ${clip.name || "图片"}`}
        >
          {source ? (
            <img
              src={source}
              alt={clip.name || "剪贴板图片"}
              className="h-full w-full object-cover transition-transform duration-200 hover:scale-[1.03]"
            />
          ) : failed ? (
            <ImageIcon className="h-5 w-5 text-muted-foreground" />
          ) : (
            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
          )}
        </button>
      </DialogTrigger>
      <DialogContent className="max-w-5xl p-3 sm:p-4">
        <DialogHeader className="pr-8">
          <DialogTitle className="truncate text-base">
            {clip.name || "图片预览"}
          </DialogTitle>
          <DialogDescription>
            {clip.mimeType || "图片"} · {formatBytes(clip.size)}
          </DialogDescription>
        </DialogHeader>
        {source && (
          <div className="flex max-h-[78vh] items-center justify-center overflow-hidden rounded-lg bg-black/95">
            <img
              src={source}
              alt={clip.name || "剪贴板图片预览"}
              className="max-h-[78vh] max-w-full object-contain"
            />
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
