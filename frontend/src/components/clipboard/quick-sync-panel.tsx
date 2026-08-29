import { useRef, useState, type ChangeEvent, type ReactNode } from "react"
import { Clipboard, ClipboardPaste, FileUp, Loader2, Send, Upload, X } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { readLocalClipboard, type ClipboardCapture } from "@/lib/clipboard"
import { cn, formatBytes, formatDuration } from "@/lib/utils"
import type { AppConfig } from "@/types"

type CaptureMode = "text" | "file"

interface QuickSyncPanelProps {
  config: AppConfig
  onSync: (capture: ClipboardCapture) => Promise<boolean>
  onError: (message: string) => void
}

export function QuickSyncPanel({ config, onSync, onError }: QuickSyncPanelProps) {
  const [mode, setMode] = useState<CaptureMode>("text")
  const [text, setText] = useState("")
  const [file, setFile] = useState<File | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  async function sync(capture: ClipboardCapture) {
    setSubmitting(true)
    try {
      return await onSync(capture)
    } finally {
      setSubmitting(false)
    }
  }

  async function syncLocalClipboard() {
    onError("")
    try {
      await sync(await readLocalClipboard())
    } catch (caught) {
      onError(caught instanceof Error
        ? caught.message
        : "浏览器拒绝读取剪贴板，请直接按 Ctrl/⌘+V 同步")
    }
  }

  async function submit() {
    if (mode === "text") {
      if (!text.trim()) return onError("请输入或粘贴文本")
      if (await sync({ kind: "text", formats: { "text/plain": text } })) setText("")
      return
    }

    if (!file) return onError("请选择要同步的文件")
    if (await sync({ kind: "file", file })) clearFile()
  }

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const selected = event.target.files?.[0] ?? null
    if (!selected) return setFile(null)
    if (selected.size > config.maxUploadBytes) {
      clearFile()
      onError(`文件不能超过 ${formatBytes(config.maxUploadBytes)}`)
      return
    }
    onError("")
    setFile(selected)
  }

  function clearFile() {
    setFile(null)
    if (fileInputRef.current) fileInputRef.current.value = ""
  }

  return (
    <Card className="overflow-hidden">
      <CardHeader className="space-y-3 border-b bg-muted/25 p-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle className="flex items-center gap-2 text-base">
              <Send className="h-4 w-4" />
              快速同步
            </CardTitle>
            <CardDescription className="mt-1">
              页面空白处使用快捷键，无需打开输入框
            </CardDescription>
          </div>
          <Button
            variant="outline"
            size="sm"
            disabled={submitting}
            onClick={() => void syncLocalClipboard()}
          >
            {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <ClipboardPaste className="h-4 w-4" />}
            读取并同步
          </Button>
        </div>

        <div className="grid grid-cols-2 gap-2 text-xs">
          <ShortcutHint keys="Ctrl / ⌘ + V" label="同步本机剪贴板" />
          <ShortcutHint keys="Ctrl / ⌘ + C" label="复制最新文本" />
        </div>
      </CardHeader>

      <CardContent className="space-y-4 p-4">
        <div className="grid grid-cols-2 rounded-lg bg-muted p-1" role="tablist" aria-label="同步内容类型">
          <ModeButton active={mode === "text"} onClick={() => setMode("text")}>
            <Clipboard className="h-4 w-4" />文本
          </ModeButton>
          <ModeButton active={mode === "file"} onClick={() => setMode("file")}>
            <FileUp className="h-4 w-4" />文件 / 图片
          </ModeButton>
        </div>

        {mode === "text" ? (
          <div className="space-y-3" role="tabpanel">
            <Label htmlFor="plain-text">剪贴板内容</Label>
            <Textarea
              id="plain-text"
              value={text}
              onChange={(event) => setText(event.target.value)}
              placeholder="粘贴或输入要同步的内容…"
              className="min-h-28 resize-y"
            />
            <Button
              onClick={() => void submit()}
              disabled={submitting || !text.trim()}
              className="w-full"
            >
              {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
              同步文本
            </Button>
          </div>
        ) : (
          <div className="space-y-3" role="tabpanel">
            <label
              htmlFor="file-input"
              className="group flex min-h-32 cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed border-input bg-muted/20 px-4 text-center transition-colors hover:border-foreground/40 hover:bg-muted/40"
            >
              <div className="mb-2 flex h-10 w-10 items-center justify-center rounded-full bg-foreground/10">
                <FileUp className="h-5 w-5" />
              </div>
              <span className="text-sm font-medium">选择图片或文件</span>
              <span className="mt-1 text-xs text-muted-foreground">
                最大 {formatBytes(config.maxUploadBytes)}，{formatDuration(config.fileTtlSeconds)} 后清理
              </span>
              <input
                ref={fileInputRef}
                id="file-input"
                type="file"
                className="sr-only"
                onChange={handleFileChange}
              />
            </label>

            {file && (
              <div className="flex items-center justify-between rounded-lg border bg-background px-3 py-2 text-sm">
                <div className="flex min-w-0 items-center gap-2">
                  <FileUp className="h-4 w-4 shrink-0" />
                  <span className="truncate">{file.name}</span>
                  <span className="shrink-0 text-xs text-muted-foreground">{formatBytes(file.size)}</span>
                </div>
                <button type="button" onClick={clearFile} className="text-muted-foreground hover:text-foreground" aria-label="移除文件">
                  <X className="h-4 w-4" />
                </button>
              </div>
            )}

            <Button onClick={() => void submit()} disabled={submitting || !file} className="w-full">
              {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
              上传并同步
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function ModeButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={cn(
        "flex h-8 items-center justify-center gap-2 rounded-md px-3 text-sm font-medium transition-colors",
        active ? "bg-background text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground",
      )}
    >
      {children}
    </button>
  )
}

function ShortcutHint({ keys, label }: { keys: string; label: string }) {
  return (
    <div className="rounded-lg border bg-background/80 px-2.5 py-2">
      <kbd className="font-mono font-semibold text-foreground">{keys}</kbd>
      <p className="mt-0.5 truncate text-muted-foreground">{label}</p>
    </div>
  )
}
