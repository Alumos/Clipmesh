import { useRef, useState, type ChangeEvent, type ReactNode } from "react"
import {
  Clipboard,
  ClipboardPaste,
  Code2,
  FileClock,
  FileText,
  FileUp,
  Loader2,
  Send,
  ShieldCheck,
  Upload,
  X,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { createText, uploadFile } from "@/lib/api"
import { formatBytes, formatDuration } from "@/lib/utils"
import type { AppConfig, Clip } from "@/types"

type CaptureMode = "text" | "file"

interface QuickSyncPanelProps {
  config: AppConfig
  clipCount: number
  deviceName: string
  onCreated: (clip: Clip) => void
  onNotice: (message: string) => void
  onError: (message: string) => void
}

export function QuickSyncPanel({
  config,
  clipCount,
  deviceName,
  onCreated,
  onNotice,
  onError,
}: QuickSyncPanelProps) {
  const [mode, setMode] = useState<CaptureMode>("text")
  const [text, setText] = useState("")
  const [html, setHTML] = useState("")
  const [file, setFile] = useState<File | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const fileTtl = formatDuration(config.fileTtlSeconds)

  async function readLocalClipboard() {
    onError("")
    try {
      if (navigator.clipboard?.read) {
        const items = await navigator.clipboard.read()
        let nextText = ""
        let nextHTML = ""
        let nextFile: File | null = null

        for (const item of items) {
          if (item.types.includes("text/plain")) {
            nextText = await (await item.getType("text/plain")).text()
          }
          if (item.types.includes("text/html")) {
            nextHTML = await (await item.getType("text/html")).text()
          }
          const binaryType = item.types.find((type) => !type.startsWith("text/"))
          if (binaryType) {
            const blob = await item.getType(binaryType)
            const subtype = binaryType.split("/")[1] ?? "bin"
            const extension = subtype.replace(/\+xml$/, "").replace(/[^a-z0-9]+/gi, "-")
            nextFile = new File(
              [blob],
              `clipboard-${Date.now()}.${extension || "bin"}`,
              { type: blob.type || binaryType },
            )
          }
        }

        if (nextFile) {
          setMode("file")
          if (chooseFile(nextFile)) {
            onNotice(`已读取 ${nextFile.type || "二进制"} 剪贴板内容，请上传同步`)
          }
          return
        }
        if (nextText || nextHTML) {
          const textFromHTML = new DOMParser()
            .parseFromString(nextHTML, "text/html")
            .body.textContent
          setText(nextText || textFromHTML || "")
          setHTML(nextHTML)
          onNotice("已读取文本与 HTML 格式")
          return
        }
      }

      setText(await navigator.clipboard.readText())
      setHTML("")
      onNotice("已读取纯文本剪贴板")
    } catch {
      onError("浏览器拒绝读取剪贴板，请点击页面后重试，或直接粘贴到输入框")
    }
  }

  async function submit() {
    setSubmitting(true)
    onError("")
    try {
      if (mode === "text") {
        if (!text.trim()) throw new Error("请输入或粘贴文本")
        const formats: Record<string, string> = { "text/plain": text }
        if (html.trim()) formats["text/html"] = html
        const created = await createText(formats, deviceName)
        setText("")
        setHTML("")
        onCreated(created)
        onNotice("文本已同步到 Clipmesh")
        return
      }

      if (!file) throw new Error("请选择要同步的文件")
      if (file.size > config.maxUploadBytes) {
        throw new Error(`文件不能超过 ${formatBytes(config.maxUploadBytes)}`)
      }
      const created = await uploadFile(file, deviceName)
      clearFile()
      onCreated(created)
      onNotice("文件已上传并开始同步")
    } catch (caught) {
      onError(caught instanceof Error ? caught.message : "提交失败")
    } finally {
      setSubmitting(false)
    }
  }

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const selected = event.target.files?.[0]
    if (!selected) {
      setFile(null)
      return
    }
    chooseFile(selected)
  }

  function chooseFile(selected: File) {
    if (selected.size > config.maxUploadBytes) {
      clearFile()
      onError(`文件不能超过 ${formatBytes(config.maxUploadBytes)}`)
      return false
    }
    onError("")
    setFile(selected)
    return true
  }

  function clearFile() {
    setFile(null)
    if (fileInputRef.current) fileInputRef.current.value = ""
  }

  return (
    <Card className="overflow-hidden">
      <CardHeader className="border-b bg-muted/25 p-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle className="flex items-center gap-2 text-base">
              <Send className="h-4 w-4" />
              快速同步
            </CardTitle>
            <CardDescription className="mt-1">
              发送文本、图片或临时文件
            </CardDescription>
          </div>
          <Badge variant="outline" className="gap-1.5 font-normal">
            <ShieldCheck className="h-3.5 w-3.5" />
            私有
          </Badge>
        </div>
      </CardHeader>

      <CardContent className="p-4">
        <Tabs value={mode} onValueChange={(value) => setMode(value as CaptureMode)}>
          <TabsList className="grid w-full grid-cols-2">
            <TabsTrigger value="text">
              <Clipboard className="mr-2 h-4 w-4" />
              文本
            </TabsTrigger>
            <TabsTrigger value="file">
              <FileUp className="mr-2 h-4 w-4" />
              文件 / 图片
            </TabsTrigger>
          </TabsList>

          <TabsContent value="text" className="space-y-3">
            <div className="flex items-center justify-between gap-2">
              <Label htmlFor="plain-text">剪贴板内容</Label>
              <Button variant="ghost" size="sm" onClick={() => void readLocalClipboard()}>
                <ClipboardPaste className="h-4 w-4" />
                读取本机
              </Button>
            </div>
            <Textarea
              id="plain-text"
              value={text}
              onChange={(event) => setText(event.target.value)}
              placeholder="粘贴或输入要同步的内容…"
              className="min-h-28 resize-y"
            />
            <details className="rounded-lg border bg-muted/20 p-3">
              <summary className="flex cursor-pointer list-none items-center gap-2 text-xs font-medium">
                <Code2 className="h-3.5 w-3.5" />
                附加 HTML 格式
                <span className="font-normal text-muted-foreground">可选</span>
              </summary>
              <Textarea
                value={html}
                onChange={(event) => setHTML(event.target.value)}
                placeholder="例如：<strong>富文本</strong>"
                className="mt-3 min-h-20 resize-y bg-background font-mono text-xs"
              />
            </details>
            <Button
              onClick={() => void submit()}
              disabled={submitting || !text.trim()}
              className="w-full"
            >
              {submitting ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Upload className="h-4 w-4" />
              )}
              同步文本
            </Button>
          </TabsContent>

          <TabsContent value="file" className="space-y-3">
            <label
              htmlFor="file-input"
              className="group flex min-h-32 cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed border-input bg-muted/20 px-4 text-center transition-colors hover:border-foreground/40 hover:bg-muted/40"
            >
              <div className="mb-2 flex h-10 w-10 items-center justify-center rounded-full bg-foreground/10">
                <FileUp className="h-5 w-5" />
              </div>
              <span className="text-sm font-medium">选择图片或文件</span>
              <span className="mt-1 text-xs text-muted-foreground">
                最大 {formatBytes(config.maxUploadBytes)}，{fileTtl} 后清理
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
                  <span className="shrink-0 text-xs text-muted-foreground">
                    {formatBytes(file.size)}
                  </span>
                </div>
                <button
                  type="button"
                  onClick={clearFile}
                  className="text-muted-foreground hover:text-foreground"
                  aria-label="移除文件"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
            )}

            <Button
              onClick={() => void submit()}
              disabled={submitting || !file}
              className="w-full"
            >
              {submitting ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Upload className="h-4 w-4" />
              )}
              上传并同步
            </Button>
          </TabsContent>
        </Tabs>
      </CardContent>

      <div className="grid grid-cols-3 border-t bg-muted/20">
        <MiniMetric
          icon={<Clipboard className="h-3.5 w-3.5" />}
          value={clipCount.toString()}
          label="当前记录"
        />
        <MiniMetric
          icon={<FileText className="h-3.5 w-3.5" />}
          value={config.textLimit.toString()}
          label="文本上限"
        />
        <MiniMetric
          icon={<FileClock className="h-3.5 w-3.5" />}
          value={fileTtl}
          label="文件保留"
        />
      </div>
    </Card>
  )
}

function MiniMetric({
  label,
  value,
  icon,
}: {
  label: string
  value: string
  icon: ReactNode
}) {
  return (
    <div className="min-w-0 border-r px-3 py-3 last:border-r-0">
      <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
        {icon}
        <span className="truncate">{label}</span>
      </div>
      <p className="mt-1 truncate text-sm font-semibold">{value}</p>
    </div>
  )
}
