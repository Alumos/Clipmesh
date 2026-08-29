import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { X } from "lucide-react"

import { AdminPage } from "@/AdminPage"
import { AppHeader, type AppView, type ConnectionState } from "@/components/app-header"
import { AuthOverlay } from "@/components/auth-overlay"
import { ClipboardHistory } from "@/components/clipboard/clipboard-history"
import { DeviceSettings } from "@/components/clipboard/device-settings"
import { QuickSyncPanel } from "@/components/clipboard/quick-sync-panel"
import {
  ApiError,
  createText,
  deleteClip,
  downloadFile,
  fetchClips,
  fetchConfig,
  fetchCurrentUser,
  login,
  logout,
  uploadFile,
} from "@/lib/api"
import {
  captureFromTransfer,
  shouldKeepNativeClipboard,
  writeClipToClipboard,
  writeClipToTransfer,
  type ClipboardCapture,
} from "@/lib/clipboard"
import { detectDevice, getDeviceName, resetDeviceName, setDeviceName } from "@/lib/device"
import { connectEvents } from "@/lib/events"
import { cn, formatBytes } from "@/lib/utils"
import type { AppConfig, Clip, ClipEvent, User } from "@/types"

type SessionStatus = "checking" | "authenticated" | "anonymous"

const defaultConfig: AppConfig = {
  textLimit: 100,
  fileTtlSeconds: 86_400,
  maxUploadBytes: 100 * 1024 * 1024,
  pageSize: 6,
}

function viewFromLocation(): AppView {
  return window.location.pathname.startsWith("/admin") ? "users" : "clipboard"
}

async function fetchWorkspace() {
  const [config, clips] = await Promise.all([fetchConfig(), fetchClips()])
  return { config, clips }
}

function App() {
  const [config, setConfig] = useState(defaultConfig)
  const [clips, setClips] = useState<Clip[]>([])
  const [loading, setLoading] = useState(true)
  const [sessionStatus, setSessionStatus] = useState<SessionStatus>("checking")
  const [authenticating, setAuthenticating] = useState(false)
  const [currentUser, setCurrentUser] = useState<User | null>(null)
  const [connection, setConnection] = useState<ConnectionState>("offline")
  const [lastSyncAt, setLastSyncAt] = useState<Date | null>(null)
  const [error, setError] = useState("")
  const [notice, setNotice] = useState("")
  const [copyingId, setCopyingId] = useState("")
  const [deletingId, setDeletingId] = useState("")
  const [deviceName, setDeviceNameDraft] = useState(getDeviceName)
  const [view, setView] = useState<AppView>(viewFromLocation)
  const deviceProfile = useMemo(() => detectDevice(), [])
  const syncingRef = useRef(false)

  const showNotice = useCallback((message: string) => {
    setError("")
    setNotice(message)
    window.setTimeout(() => {
      setNotice((current) => current === message ? "" : current)
    }, 2800)
  }, [])

  const handleRequestError = useCallback((caught: unknown) => {
    if (caught instanceof ApiError && caught.status === 401) {
      setSessionStatus("anonymous")
      setCurrentUser(null)
      setClips([])
      setError("请先登录 Clipmesh")
      return
    }
    setError(caught instanceof Error ? caught.message : "无法连接 Clipmesh")
  }, [])

  const syncCapture = useCallback(async (capture: ClipboardCapture) => {
    if (syncingRef.current) {
      showNotice("上一条剪贴板正在同步")
      return false
    }

    syncingRef.current = true
    setError("")
    try {
      const created = capture.kind === "text"
        ? await createText(capture.formats, deviceName)
        : await uploadCaptureFile(capture.file, config.maxUploadBytes, deviceName)
      setClips((current) => upsertClip(current, created))
      showNotice(capture.kind === "text" ? "文本已同步到 Clipmesh" : "文件已上传并开始同步")
      return true
    } catch (caught) {
      handleRequestError(caught)
      return false
    } finally {
      syncingRef.current = false
    }
  }, [config.maxUploadBytes, deviceName, handleRequestError, showNotice])

  const initialize = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const [user, workspace] = await Promise.all([fetchCurrentUser(), fetchWorkspace()])
      setCurrentUser(user)
      setConfig(workspace.config)
      setClips(workspace.clips)
      setSessionStatus("authenticated")
    } catch (caught) {
      handleRequestError(caught)
    } finally {
      setLoading(false)
    }
  }, [handleRequestError])

  const refreshClips = useCallback(async (showLoading = false) => {
    if (showLoading) setLoading(true)
    setError("")
    try {
      setClips(await fetchClips())
    } catch (caught) {
      handleRequestError(caught)
    } finally {
      if (showLoading) setLoading(false)
    }
  }, [handleRequestError])

  useEffect(() => {
    void initialize()
  }, [initialize])

  useEffect(() => {
    const handleNavigation = () => setView(viewFromLocation())
    window.addEventListener("popstate", handleNavigation)
    return () => window.removeEventListener("popstate", handleNavigation)
  }, [])

  useEffect(() => {
    if (sessionStatus === "authenticated" && view === "users" && currentUser?.role !== "admin") {
      window.history.replaceState({}, "", "/")
      setView("clipboard")
    }
  }, [currentUser, sessionStatus, view])

  useEffect(() => {
    if (sessionStatus !== "authenticated") {
      setConnection("offline")
      return
    }

    setConnection("connecting")
    const source = connectEvents((event: ClipEvent) => {
      setConnection("connected")
      setLastSyncAt(new Date())
      if (event.type === "created" && event.clip) {
        setClips((current) => upsertClip(current, event.clip as Clip))
      }
      if (event.type === "deleted" && event.id) {
        setClips((current) => current.filter((clip) => clip.id !== event.id))
      }
    })
    source.onopen = () => {
      setConnection("connected")
      // Reconcile events missed while the process or browser was offline.
      void refreshClips()
    }
    source.onerror = () => setConnection("reconnecting")
    source.onreconnecting = () => setConnection("reconnecting")
    return () => source.close()
  }, [refreshClips, sessionStatus])

  useEffect(() => {
    if (sessionStatus !== "authenticated" || view !== "clipboard") return

    const handlePaste = (event: ClipboardEvent) => {
      if (shouldKeepNativeClipboard(event.target)) return
      const capture = captureFromTransfer(event.clipboardData)
      if (!capture) return
      event.preventDefault()
      void syncCapture(capture)
    }

    const handleCopyShortcut = (event: ClipboardEvent) => {
      if (shouldKeepNativeClipboard(event.target)) return
      const latestText = clips.find((clip) => clip.kind === "text")
      if (!latestText || !writeClipToTransfer(event.clipboardData, latestText)) {
        showNotice("暂无可复制的文本记录")
        return
      }
      event.preventDefault()
      showNotice("已复制最新文本到本机剪贴板")
    }

    window.addEventListener("paste", handlePaste)
    window.addEventListener("copy", handleCopyShortcut)
    return () => {
      window.removeEventListener("paste", handlePaste)
      window.removeEventListener("copy", handleCopyShortcut)
    }
  }, [clips, sessionStatus, showNotice, syncCapture, view])

  function navigate(nextView: AppView) {
    const nextPath = nextView === "users" ? "/admin/users" : "/"
    if (window.location.pathname !== nextPath) {
      window.history.pushState({}, "", nextPath)
    }
    setView(nextView)
  }

  async function handleLogin(username: string, password: string) {
    setAuthenticating(true)
    setError("")
    try {
      const user = await login(username, password)
      const workspace = await fetchWorkspace()
      setCurrentUser(user)
      setConfig(workspace.config)
      setClips(workspace.clips)
      setSessionStatus("authenticated")
      showNotice("登录成功，已开始同步")
      return true
    } catch (caught) {
      setSessionStatus("anonymous")
      setCurrentUser(null)
      setError(caught instanceof Error ? caught.message : "登录失败")
      return false
    } finally {
      setAuthenticating(false)
      setLoading(false)
    }
  }

  async function handleLogout() {
    try {
      await logout()
    } catch {
      // The browser still returns to a signed-out state if the server session expired.
    }
    setSessionStatus("anonymous")
    setCurrentUser(null)
    setClips([])
    setConnection("offline")
  }

  async function handleCopy(clip: Clip) {
    setCopyingId(clip.id)
    setError("")
    try {
      await writeClipToClipboard(clip)
      showNotice("已复制到本机剪贴板")
    } catch {
      setError("复制失败，请确认当前页面处于 HTTPS 或 localhost 安全环境")
    } finally {
      setCopyingId("")
    }
  }

  async function handleDownload(clip: Clip) {
    setError("")
    try {
      const blob = await downloadFile(clip.id)
      const objectUrl = URL.createObjectURL(blob)
      const anchor = document.createElement("a")
      anchor.href = objectUrl
      anchor.download = clip.name ?? "clipmesh-file"
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000)
      showNotice("文件下载已开始")
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "文件下载失败")
    }
  }

  async function handleDelete(clip: Clip) {
    setDeletingId(clip.id)
    setError("")
    try {
      await deleteClip(clip.id)
      setClips((current) => current.filter((item) => item.id !== clip.id))
      showNotice("已删除这条剪贴板记录")
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "删除失败")
    } finally {
      setDeletingId("")
    }
  }

  function saveDeviceSettings() {
    const normalizedName = deviceName.trim() || deviceProfile.label
    setDeviceNameDraft(normalizedName)
    setDeviceName(normalizedName)
    showNotice("设备设置已保存")
  }

  function restoreDeviceSettings() {
    setDeviceNameDraft(resetDeviceName())
    showNotice("已恢复 UA 自动识别名称")
  }

  const adminView = view === "users" && currentUser?.role === "admin"

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top_right,hsl(var(--foreground)/0.06),transparent_30rem)]">
      <AppHeader
        view={adminView ? "users" : "clipboard"}
        user={currentUser}
        connection={connection}
        lastSyncAt={lastSyncAt}
        loading={loading}
        onNavigate={navigate}
        onRefresh={() => void refreshClips(true)}
        onLogout={() => void handleLogout()}
      />

      {adminView ? (
        <AdminPage
          currentUser={currentUser}
          onNotice={showNotice}
          onError={setError}
        />
      ) : (
        <main className="container py-4 sm:py-6">
          {(error || notice) && (
            <div
              className={cn(
                "mb-4 flex items-start justify-between gap-3 rounded-lg border px-4 py-3 text-sm",
                error ? "border-foreground/20 bg-foreground/[0.06]" : "border-border bg-muted",
              )}
            >
              <span>{error || notice}</span>
              <button
                type="button"
                className="shrink-0 text-muted-foreground hover:text-foreground"
                onClick={() => {
                  setError("")
                  setNotice("")
                }}
                aria-label="关闭提示"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          )}

          <section className="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_340px] lg:gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
            <ClipboardHistory
              clips={clips}
              loading={loading}
              pageSize={config.pageSize || 6}
              copyingId={copyingId}
              deletingId={deletingId}
              onCopy={(clip) => void handleCopy(clip)}
              onDownload={(clip) => void handleDownload(clip)}
              onDelete={(clip) => void handleDelete(clip)}
              onNew={() => {
                document.getElementById("quick-sync")?.scrollIntoView({
                  behavior: "smooth",
                  block: "start",
                })
              }}
            />

            <aside id="quick-sync" className="scroll-mt-24 space-y-4 lg:sticky lg:top-24">
              <QuickSyncPanel
                config={config}
                onSync={syncCapture}
                onError={setError}
              />
              <DeviceSettings
                name={deviceName}
                profile={deviceProfile}
                onNameChange={setDeviceNameDraft}
                onSave={saveDeviceSettings}
                onRestore={restoreDeviceSettings}
              />
            </aside>
          </section>
        </main>
      )}

      {sessionStatus === "anonymous" && (
        <AuthOverlay loading={authenticating} onLogin={handleLogin} />
      )}
    </div>
  )
}

function upsertClip(clips: Clip[], clip: Clip) {
  return [clip, ...clips.filter((item) => item.id !== clip.id)]
}

function uploadCaptureFile(file: File, maxUploadBytes: number, deviceName: string) {
  if (file.size > maxUploadBytes) {
    throw new Error(`文件不能超过 ${formatBytes(maxUploadBytes)}`)
  }
  return uploadFile(file, deviceName)
}

export default App
