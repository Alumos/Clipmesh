import { clipPlainText } from "@/lib/clips"
import type { Clip } from "@/types"

export type ClipboardCapture =
  | { kind: "text"; formats: Record<string, string> }
  | { kind: "file"; file: File }

export function captureFromTransfer(data: DataTransfer | null): ClipboardCapture | null {
  if (!data) return null

  const transferredFile = Array.from(data.files)[0]
    ?? Array.from(data.items).find((item) => item.kind === "file")?.getAsFile()
  if (transferredFile) return { kind: "file", file: normalizeFile(transferredFile) }

  const html = data.getData("text/html")
  const plain = data.getData("text/plain") || textFromHTML(html)
  if (!plain.trim()) return null

  const formats: Record<string, string> = { "text/plain": plain }
  if (html.trim()) formats["text/html"] = html
  return { kind: "text", formats }
}

export async function readLocalClipboard(): Promise<ClipboardCapture> {
  if (navigator.clipboard?.read) {
    const items = await navigator.clipboard.read()
    let plain = ""
    let html = ""

    for (const item of items) {
      const binaryType = item.types.find((type) => !type.startsWith("text/"))
      if (binaryType) {
        const blob = await item.getType(binaryType)
        return { kind: "file", file: fileFromBlob(blob, binaryType) }
      }
      if (item.types.includes("text/plain")) {
        plain = await (await item.getType("text/plain")).text()
      }
      if (item.types.includes("text/html")) {
        html = await (await item.getType("text/html")).text()
      }
    }

    plain ||= textFromHTML(html)
    if (plain.trim()) {
      const formats: Record<string, string> = { "text/plain": plain }
      if (html.trim()) formats["text/html"] = html
      return { kind: "text", formats }
    }
  }

  if (!navigator.clipboard?.readText) {
    throw new Error("当前浏览器不允许主动读取剪贴板，请直接按 Ctrl/⌘+V 同步")
  }
  const plain = await navigator.clipboard.readText()
  if (!plain.trim()) throw new Error("本机剪贴板中没有可同步的内容")
  return { kind: "text", formats: { "text/plain": plain } }
}

export async function writeClipToClipboard(clip: Clip) {
  const plain = clipPlainText(clip)
  const html = clip.formats?.["text/html"]
  if (html && navigator.clipboard.write && typeof ClipboardItem !== "undefined") {
    await navigator.clipboard.write([
      new ClipboardItem({
        "text/plain": new Blob([plain], { type: "text/plain" }),
        "text/html": new Blob([html], { type: "text/html" }),
      }),
    ])
    return
  }
  await navigator.clipboard.writeText(plain)
}

export function writeClipToTransfer(data: DataTransfer | null, clip: Clip) {
  if (!data || clip.kind !== "text") return false
  const plain = clipPlainText(clip)
  if (!plain) return false
  data.setData("text/plain", plain)
  const html = clip.formats?.["text/html"]
  if (html) data.setData("text/html", html)
  return true
}

export function shouldKeepNativeClipboard(target: EventTarget | null) {
  if (window.getSelection()?.toString()) return true
  if (!(target instanceof Element)) return false
  return Boolean(target.closest("input, textarea, select, [contenteditable]:not([contenteditable='false']), [role='textbox']"))
}

function normalizeFile(file: File) {
  if (file.name.trim()) return file
  return fileFromBlob(file, file.type)
}

function fileFromBlob(blob: Blob, mimeType: string) {
  const subtype = mimeType.split("/")[1] ?? "bin"
  const extension = subtype.replace(/\+xml$/, "").replace(/[^a-z0-9]+/gi, "-") || "bin"
  return new File([blob], `clipboard-${Date.now()}.${extension}`, {
    type: blob.type || mimeType || "application/octet-stream",
  })
}

function textFromHTML(html: string) {
  if (!html) return ""
  return new DOMParser().parseFromString(html, "text/html").body.textContent ?? ""
}
