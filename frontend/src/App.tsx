import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import {
  Check,
  Clipboard,
  ClipboardPaste,
  Cloud,
  Code2,
  Copy,
  Download,
  FileClock,
  FileText,
  FileUp,
  KeyRound,
  Laptop,
  Loader2,
  LogOut,
  LockKeyhole,
  MonitorSmartphone,
  RefreshCw,
  RotateCcw,
  Search,
  Send,
  Settings2,
  ShieldCheck,
  Smartphone,
  Trash2,
  Upload,
  Users,
  Wifi,
  WifiOff,
  X,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import {
  ApiError,
  connectEvents,
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
import { detectDevice, getDeviceName, resetDeviceName, setDeviceName } from "@/lib/device"
import { cn, formatBytes, formatDuration, formatRelativeTime } from "@/lib/utils"
import type { AppConfig, Clip, ClipEvent, DeviceProfile, User } from "@/types"
import { AdminPage } from "@/AdminPage"

type CaptureMode = "text" | "file"
type FilterKind = "all" | "text" | "file"
type AppView = "clipboard" | "users"
type ConnectionState = "offline" | "connecting" | "connected" | "reconnecting"

function viewFromLocation(): AppView {
  return window.location.pathname.startsWith("/admin") ? "users" : "clipboard"
}

const defaultConfig: AppConfig = {
  textLimit: 100,
  fileTtlSeconds: 86_400,
  maxUploadBytes: 100 * 1024 * 1024,
  authEnabled: true,
  pageSize: 6,
}

function App() {
  const [config, setConfig] = useState(defaultConfig)
  const [clips, setClips] = useState<Clip[]>([])
  const [loading, setLoading] = useState(true)
  const [connectionState, setConnectionState] = useState<ConnectionState>("offline")
  const [lastSyncAt, setLastSyncAt] = useState<Date | null>(null)
  const [error, setError] = useState("")
  const [notice, setNotice] = useState("")
  const [authenticated, setAuthenticated] = useState(false)
  const [authRequired, setAuthRequired] = useState(false)
  const [loginUsername, setLoginUsername] = useState(() => localStorage.getItem("clipmesh-login-username") ?? "admin")
  const [loginPassword, setLoginPassword] = useState("")
  const [authenticating, setAuthenticating] = useState(false)
  const [currentUser, setCurrentUser] = useState<User | null>(null)
  const [deviceName, setDeviceNameState] = useState(getDeviceName())
  const [deviceProfile, setDeviceProfile] = useState<DeviceProfile>(() => detectDevice())
  const [captureMode, setCaptureMode] = useState<CaptureMode>("text")
  const [text, setText] = useState("")
  const [html, setHtml] = useState("")
  const [file, setFile] = useState<File | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [submitting, setSubmitting] = useState(false)
  const [search, setSearch] = useState("")
  const [filter, setFilter] = useState<FilterKind>("all")
  const [page, setPage] = useState(1)
  const [copying, setCopying] = useState("")
  const [deleting, setDeleting] = useState("")
  const [view, setView] = useState<AppView>(() => viewFromLocation())

  const load = useCallback(async (showLoading = false) => {
    if (showLoading) setLoading(true)
    setError("")
    try {
      const [nextConfig, nextClips, user] = await Promise.all([fetchConfig(), fetchClips(), fetchCurrentUser()])
      setConfig(nextConfig)
      setClips(nextClips)
      setCurrentUser(user)
      setAuthenticated(true)
      setAuthRequired(false)
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 401) {
        setAuthenticated(false)
        setCurrentUser(null)
        setAuthRequired(true)
        setClips([])
        setError("请先登录 Clipmesh")
      } else {
        setError(caught instanceof Error ? caught.message : "无法连接 Clipmesh")
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load(true)
  }, [load])

  useEffect(() => {
    const handleNavigation = () => setView(viewFromLocation())
    window.addEventListener("popstate", handleNavigation)
    return () => window.removeEventListener("popstate", handleNavigation)
  }, [])

  function navigate(nextView: AppView) {
    const nextPath = nextView === "users" ? "/admin/users" : "/"
    if (window.location.pathname !== nextPath) window.history.pushState({}, "", nextPath)
    setView(nextView)
  }

  useEffect(() => {
    if (!authenticated) {
      setConnectionState("offline")
      return
    }
    setConnectionState("connecting")
    const source = connectEvents((event: ClipEvent) => {
      setConnectionState("connected")
      setLastSyncAt(new Date())
      if (event.type === "created" && event.clip) {
        setClips((current) => [event.clip as Clip, ...current.filter((clip) => clip.id !== event.clip?.id)])
        void load()
      }
      if (event.type === "deleted" && event.id) setClips((current) => current.filter((clip) => clip.id !== event.id))
    })
    source.onopen = () => {
      setConnectionState("connected")
      void load()
    }
    source.onerror = () => setConnectionState("reconnecting")
    source.onreconnecting = () => setConnectionState("reconnecting")
    return () => source.close()
  }, [authenticated, load])

  const visibleClips = useMemo(() => {
    const normalizedSearch = search.trim().toLowerCase()
    return clips.filter((clip) => {
      if (filter !== "all" && clip.kind !== filter) return false
      if (!normalizedSearch) return true
      const content = clip.kind === "text" ? plainText(clip) : `${clip.name ?? ""} ${clip.mimeType ?? ""}`
      return `${content} ${clip.deviceName}`.toLowerCase().includes(normalizedSearch)
    })
  }, [clips, filter, search])

  const pageSize = Math.max(1, config.pageSize || 6)
  const pageCount = Math.max(1, Math.ceil(visibleClips.length / pageSize))
  const pageClips = visibleClips.slice((page - 1) * pageSize, page * pageSize)
  useEffect(() => {
    setPage(1)
  }, [filter, search])

  useEffect(() => {
    if (page > pageCount) setPage(pageCount)
  }, [page, pageCount])

  async function handleLogin() {
    setAuthenticating(true)
    setError("")
    try {
      const normalizedUsername = loginUsername.trim()
      const user = await login(normalizedUsername, loginPassword)
      localStorage.setItem("clipmesh-login-username", normalizedUsername)
      setCurrentUser(user)
      setAuthenticated(true)
      setAuthRequired(false)
      setLoginPassword("")
      await load(true)
      showNotice("登录成功，已开始同步")
    } catch (caught) {
      setAuthenticated(false)
      setAuthRequired(true)
      setError(caught instanceof Error ? caught.message : "登录失败")
    } finally {
      setAuthenticating(false)
    }
  }

  async function handleLogout() {
    try {
      await logout()
    } catch {
      // The local view is cleared even if the server session already expired.
    }
    setAuthenticated(false)
    setAuthRequired(true)
    setCurrentUser(null)
    setLoginPassword("")
    setClips([])
  }

  async function handlePaste() {
    try {
      if (navigator.clipboard?.read) {
        const items = await navigator.clipboard.read()
        let nextText = ""
        let nextHtml = ""
        let nextFile: File | null = null
        for (const item of items) {
          if (item.types.includes("text/plain")) nextText = await (await item.getType("text/plain")).text()
          if (item.types.includes("text/html")) nextHtml = await (await item.getType("text/html")).text()
          const binaryType = item.types.find((type) => !type.startsWith("text/"))
          if (binaryType) {
            const blob = await item.getType(binaryType)
            const extension = binaryType.split("/")[1]?.replace(/\+xml$/, "") || "bin"
            nextFile = new File([blob], `clipboard-${Date.now()}.${extension}`, { type: blob.type || binaryType })
          }
        }
        if (nextFile) {
          setFile(nextFile)
          setCaptureMode("file")
          showNotice(`已读取 ${nextFile.type || "二进制"} 剪贴板内容，请上传同步`)
          return
        }
        if (nextText || nextHtml) {
          const plain = nextText || new DOMParser().parseFromString(nextHtml, "text/html").body.textContent || ""
          setText(plain)
          setHtml(nextHtml)
          showNotice("已读取文本与 HTML 格式")
          return
        }
      }
      setText(await navigator.clipboard.readText())
      setHtml("")
      showNotice("已读取纯文本剪贴板")
    } catch {
      setError("浏览器拒绝读取剪贴板，请点击页面后重试，或直接粘贴到输入框")
    }
  }

  async function handleSubmit() {
    setSubmitting(true)
    setError("")
    try {
      if (captureMode === "text") {
        if (!text.trim()) throw new Error("请输入或粘贴文本")
        const formats: Record<string, string> = { "text/plain": text }
        if (html.trim()) formats["text/html"] = html
        const created = await createText(formats, deviceName)
        setClips((current) => [created, ...current.filter((clip) => clip.id !== created.id)])
        setText("")
        setHtml("")
        void load()
        showNotice("文本已同步到 Clipmesh")
      } else {
        if (!file) throw new Error("请选择要同步的文件")
        const created = await uploadFile(file, deviceName)
        setClips((current) => [created, ...current.filter((clip) => clip.id !== created.id)])
        setFile(null)
        if (fileInputRef.current) fileInputRef.current.value = ""
        showNotice("文件已上传并开始同步")
        void load()
      }
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "提交失败")
    } finally {
      setSubmitting(false)
    }
  }

  async function handleCopy(clip: Clip) {
    setCopying(clip.id)
    try {
      const formats = clip.formats ?? {}
      const plain = formats["text/plain"] ?? clip.preview ?? ""
      const rich = formats["text/html"]
      if (rich && navigator.clipboard.write && typeof ClipboardItem !== "undefined") {
        await navigator.clipboard.write([new ClipboardItem({ "text/plain": new Blob([plain], { type: "text/plain" }), "text/html": new Blob([rich], { type: "text/html" }) })])
      } else {
        await navigator.clipboard.writeText(plain)
      }
      showNotice("已复制到本机剪贴板")
    } catch {
      setError("复制失败，请确认当前页面处于 HTTPS 或 localhost 安全环境")
    } finally {
      setCopying("")
    }
  }

  async function handleDownload(clip: Clip) {
    try {
      const blob = await downloadFile(clip.id)
      const objectUrl = URL.createObjectURL(blob)
      const anchor = document.createElement("a")
      anchor.href = objectUrl
      anchor.download = clip.name ?? "clipmesh-file"
      anchor.click()
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000)
      showNotice("文件下载已开始")
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "文件下载失败")
    }
  }

  async function handleDelete(clip: Clip) {
    setDeleting(clip.id)
    try {
      await deleteClip(clip.id)
      setClips((current) => current.filter((item) => item.id !== clip.id))
      showNotice("已删除这条剪贴板记录")
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "删除失败")
    } finally {
      setDeleting("")
    }
  }

  function saveSettings() {
    const normalizedName = deviceName.trim() || deviceProfile.label
    setDeviceNameState(normalizedName)
    setDeviceName(normalizedName)
    showNotice("设备设置已保存")
  }

  function restoreDetectedName() {
    const automaticName = resetDeviceName()
    setDeviceNameState(automaticName)
    showNotice("已恢复 UA 自动识别名称")
  }

  function showNotice(message: string) {
    setNotice(message)
    window.setTimeout(() => setNotice((current) => (current === message ? "" : current)), 2800)
  }

  const fileTtl = formatDuration(config.fileTtlSeconds)
  const connectionLabel = connectionState === "connected" ? "实时同步" : connectionState === "connecting" ? "连接中" : connectionState === "reconnecting" ? "重连中" : "未连接"
  const connectionTitle = lastSyncAt ? `最近事件：${lastSyncAt.toLocaleTimeString("zh-CN")}` : connectionLabel

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top_right,hsl(var(--foreground)/0.06),transparent_30rem)]">
      <header className="sticky top-0 z-30 border-b bg-background/90 backdrop-blur-xl">
        <div className="container flex min-h-16 items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary text-primary-foreground">
              <Clipboard className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="truncate text-sm font-semibold tracking-tight sm:text-base">Clipmesh</span>
                <Badge variant="outline" className="hidden sm:inline-flex">PRIVATE SYNC</Badge>
              </div>
              <p className="hidden text-xs text-muted-foreground sm:block">你的私有跨设备剪贴板</p>
            </div>
          </div>
          <nav aria-label="主导航" className="hidden items-center gap-1 rounded-lg border bg-muted/50 p-1 md:flex">
            <NavItem active={view === "clipboard"} onClick={() => navigate("clipboard")}><Clipboard className="h-4 w-4" />剪贴板</NavItem>
            {currentUser?.role === "admin" && <NavItem active={view === "users"} onClick={() => navigate("users")}><Users className="h-4 w-4" />用户与权限</NavItem>}
          </nav>
          <div className="flex items-center gap-1.5 sm:gap-2">
            <div title={connectionTitle} className={cn("flex items-center gap-1.5 rounded-full border px-2.5 py-1.5 text-xs sm:px-3", connectionState === "connected" ? "text-foreground" : "text-muted-foreground")}>
              {connectionState === "connected" ? <Wifi className="h-3.5 w-3.5" /> : connectionState === "reconnecting" ? <RefreshCw className="h-3.5 w-3.5 animate-spin" /> : <WifiOff className="h-3.5 w-3.5" />}
              <span className="hidden sm:inline">{connectionLabel}</span>
            </div>
            <Button variant="ghost" size="icon" aria-label="刷新剪贴板" onClick={() => void load(true)}><RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} /></Button>
            <div className="hidden h-8 w-px bg-border sm:block" />
            <span className="hidden max-w-28 truncate text-xs text-muted-foreground sm:block">{currentUser?.username}</span>
            <Button variant="ghost" size="icon" aria-label="退出登录" onClick={() => void handleLogout()}><LogOut className="h-4 w-4" /></Button>
          </div>
        </div>
      </header>

      <div className="border-b bg-background md:hidden">
        <nav aria-label="移动端主导航" className="container flex gap-1 py-2">
          <NavItem active={view === "clipboard"} onClick={() => navigate("clipboard")}><Clipboard className="h-4 w-4" />剪贴板</NavItem>
          {currentUser?.role === "admin" && <NavItem active={view === "users"} onClick={() => navigate("users")}><Users className="h-4 w-4" />用户与权限</NavItem>}
        </nav>
      </div>

      {view === "users" && currentUser?.role === "admin" ? <AdminPage currentUser={currentUser} onNotice={showNotice} onError={setError} /> : <main className="container space-y-5 py-5 sm:space-y-6 sm:py-8">
        <section className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(360px,0.62fr)] lg:items-end">
          <div className="space-y-3">
            <Badge variant="secondary" className="gap-1.5"><Cloud className="h-3.5 w-3.5" />数据留在你的 NAS</Badge>
            <h1 className="max-w-3xl text-3xl font-semibold leading-tight tracking-tight sm:text-4xl lg:text-[2.85rem]">复制一次，<span className="text-muted-foreground">随时随地</span>继续工作。</h1>
            <p className="max-w-2xl text-sm leading-6 text-muted-foreground sm:text-base">文本、富文本和临时文件集中在你的私有空间。打开任意设备的浏览器，即刻接上刚才的工作流。</p>
          </div>
          <div className="grid grid-cols-3 gap-2 sm:gap-3">
            <Metric label="当前记录" value={clips.length.toString()} icon={<Check className="h-4 w-4" />} />
            <Metric label="文本上限" value={`${config.textLimit} 条`} icon={<FileText className="h-4 w-4" />} />
            <Metric label="文件保留" value={fileTtl} icon={<FileClock className="h-4 w-4" />} />
          </div>
        </section>

        {(error || notice) && <div className={cn("flex items-start justify-between gap-3 rounded-lg border px-4 py-3 text-sm", error ? "border-foreground/20 bg-foreground/[0.06]" : "border-border bg-muted")}><span>{error || notice}</span><button className="shrink-0 text-muted-foreground hover:text-foreground" onClick={() => { setError(""); setNotice("") }} aria-label="关闭提示"><X className="h-4 w-4" /></button></div>}

        <section className="grid items-start gap-5 lg:grid-cols-[minmax(0,0.92fr)_minmax(0,1.08fr)] lg:gap-6">
          <div className="space-y-5 lg:sticky lg:top-24">
            <Card className="overflow-hidden shadow-sm">
              <CardHeader className="border-b bg-muted/35 pb-4 sm:flex-row sm:items-start sm:justify-between sm:space-y-0">
                <div><CardTitle className="flex items-center gap-2 text-base"><Send className="h-4 w-4" />快速同步</CardTitle><CardDescription className="mt-1">从这里发出新的剪贴板内容</CardDescription></div>
                <Badge variant="outline" className="mt-3 w-fit gap-1.5 font-normal sm:mt-0"><ShieldCheck className="h-3.5 w-3.5" />账号隔离</Badge>
              </CardHeader>
              <CardContent className="pt-5">
                <Tabs value={captureMode} onValueChange={(value) => setCaptureMode(value as CaptureMode)}>
                  <TabsList className="grid w-full grid-cols-2">
                    <TabsTrigger value="text"><Clipboard className="mr-2 h-4 w-4" />文本 / 富文本</TabsTrigger>
                    <TabsTrigger value="file"><FileUp className="mr-2 h-4 w-4" />临时文件</TabsTrigger>
                  </TabsList>
                  <TabsContent value="text" className="space-y-3">
                    <div className="flex items-center justify-between gap-2"><Label htmlFor="plain-text">纯文本内容 <span className="text-muted-foreground">*</span></Label><Button variant="outline" size="sm" onClick={() => void handlePaste()}><ClipboardPaste className="h-4 w-4" />读取剪贴板</Button></div>
                    <Textarea id="plain-text" value={text} onChange={(event) => setText(event.target.value)} placeholder="粘贴或输入要同步的内容…" className="min-h-36 resize-y" />
                    <details className="rounded-lg border bg-muted/25 p-3"><summary className="flex cursor-pointer list-none items-center gap-2 text-sm font-medium"><Code2 className="h-4 w-4" />附加 HTML 格式 <span className="font-normal text-muted-foreground">（可选）</span></summary><Textarea value={html} onChange={(event) => setHtml(event.target.value)} placeholder="例如：<strong>富文本</strong>" className="mt-3 min-h-24 resize-y bg-background font-mono text-xs" /></details>
                    <div className="flex flex-col gap-3 pt-1 sm:flex-row sm:items-center sm:justify-between"><p className="text-xs leading-5 text-muted-foreground">支持 text/plain、text/html，服务端保留最近 {config.textLimit} 条。</p><Button onClick={() => void handleSubmit()} disabled={submitting} className="w-full sm:w-auto">{submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}同步文本</Button></div>
                  </TabsContent>
                  <TabsContent value="file" className="space-y-4">
                    <label htmlFor="file-input" className="group flex min-h-44 cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed border-input bg-muted/20 px-6 text-center transition-colors hover:border-foreground/40 hover:bg-muted/40"><div className="mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-foreground/10"><FileUp className="h-6 w-6" /></div><span className="text-sm font-medium">点击选择文件</span><span className="mt-1 text-xs text-muted-foreground">单个文件最大 {formatBytes(config.maxUploadBytes)}，上传后 {fileTtl} 自动清理</span><input ref={fileInputRef} id="file-input" type="file" className="sr-only" onChange={(event) => setFile(event.target.files?.[0] ?? null)} /></label>
                    {file && <div className="flex items-center justify-between rounded-lg border bg-background px-3 py-2 text-sm"><div className="flex min-w-0 items-center gap-2"><FileUp className="h-4 w-4 shrink-0" /><span className="truncate">{file.name}</span><span className="shrink-0 text-xs text-muted-foreground">{formatBytes(file.size)}</span></div><button onClick={() => { setFile(null); if (fileInputRef.current) fileInputRef.current.value = "" }} className="text-muted-foreground hover:text-foreground" aria-label="移除文件"><X className="h-4 w-4" /></button></div>}
                    <div className="flex justify-end"><Button onClick={() => void handleSubmit()} disabled={submitting || !file} className="w-full sm:w-auto">{submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}上传并同步</Button></div>
                  </TabsContent>
                </Tabs>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-4"><CardTitle className="flex items-center gap-2 text-base"><Settings2 className="h-4 w-4" />设备设置</CardTitle><CardDescription>自动读取 UA，也可以保留自己的易读名称</CardDescription></CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center gap-3 rounded-lg border bg-muted/30 p-3"><div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-background">{deviceProfile.device === "桌面设备" ? <Laptop className="h-4 w-4" /> : <Smartphone className="h-4 w-4" />}</div><div className="min-w-0"><p className="truncate text-sm font-medium">{deviceProfile.device}</p><p className="truncate text-xs text-muted-foreground">{deviceProfile.os} · {deviceProfile.browser}</p></div><Badge variant="outline" className="ml-auto shrink-0 text-[10px]">UA</Badge></div>
                <div className="space-y-2"><Label htmlFor="device-name">当前设备名称</Label><Input id="device-name" value={deviceName} onChange={(event) => setDeviceNameState(event.target.value)} placeholder="例如：办公室 Mac" /><p className="text-xs leading-5 text-muted-foreground">此名称会显示在所有设备的最近记录中。</p></div>
                <div className="flex flex-col gap-2 sm:flex-row"><Button variant="outline" className="flex-1" onClick={saveSettings}><Settings2 className="h-4 w-4" />保存设备名称</Button><Button variant="ghost" onClick={restoreDetectedName}><RotateCcw className="h-4 w-4" />恢复自动</Button></div>
                <div className="rounded-lg bg-muted/60 p-3 text-xs leading-5 text-muted-foreground"><p className="font-medium text-foreground">安全提示</p><p className="mt-1">数据按账号完全隔离；公网部署请在反向代理开启 HTTPS，并将 Cookie Secure 设为 true。</p></div>
              </CardContent>
            </Card>
          </div>

          <section className="min-w-0">
            <Card className="overflow-hidden">
              <CardHeader className="gap-4 border-b pb-4"><div className="flex items-start justify-between gap-3"><div><CardTitle className="flex items-center gap-2 text-base"><MonitorSmartphone className="h-4 w-4" />最近剪贴板</CardTitle><CardDescription className="mt-1">只显示当前账号的同步记录</CardDescription></div><Badge variant="secondary">{visibleClips.length} 条</Badge></div><div className="flex flex-col gap-2 sm:flex-row"><div className="relative min-w-0 flex-1"><Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" /><Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索内容、设备或文件名" className="pl-9" /></div><div className="flex shrink-0 rounded-md border bg-background p-1">{(["all", "text", "file"] as FilterKind[]).map((item) => <button key={item} onClick={() => setFilter(item)} className={cn("rounded px-2.5 py-1 text-xs transition-colors", filter === item ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground")}>{item === "all" ? "全部" : item === "text" ? "文本" : "文件"}</button>)}</div></div></CardHeader>
              <CardContent className="p-4 sm:p-5">{loading ? <LoadingState /> : pageClips.length === 0 ? <EmptyState hasSearch={Boolean(search || filter !== "all")} /> : <div className="grid gap-3">{pageClips.map((clip) => <ClipCard key={clip.id} clip={clip} copying={copying === clip.id} deleting={deleting === clip.id} onCopy={() => void handleCopy(clip)} onDownload={() => void handleDownload(clip)} onDelete={() => void handleDelete(clip)} />)}</div>}{pageClips.length > 0 && <Pagination page={page} pageCount={pageCount} onPageChange={setPage} />}</CardContent>
            </Card>
          </section>
        </section>
      </main>}

      {authRequired && <AuthOverlay username={loginUsername} password={loginPassword} setUsername={setLoginUsername} setPassword={setLoginPassword} loading={authenticating} onLogin={() => void handleLogin()} />}
    </div>
  )
}

function NavItem({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return <button type="button" onClick={onClick} className={cn("inline-flex h-9 items-center justify-center gap-2 rounded-md px-3 text-sm font-medium transition-colors", active ? "bg-background text-foreground shadow-sm" : "text-muted-foreground hover:bg-background/70 hover:text-foreground")}>{children}</button>
}

function Metric({ label, value, icon }: { label: string; value: string; icon: ReactNode }) {
  return <div className="rounded-xl border bg-card p-3 shadow-sm sm:p-4"><div className="mb-2 flex h-7 w-7 items-center justify-center rounded-lg bg-muted text-foreground">{icon}</div><p className="truncate text-[11px] text-muted-foreground sm:text-xs">{label}</p><p className="mt-1 truncate text-sm font-semibold sm:text-base">{value}</p></div>
}

function ClipCard({ clip, copying, deleting, onCopy, onDownload, onDelete }: { clip: Clip; copying: boolean; deleting: boolean; onCopy: () => void; onDownload: () => void; onDelete: () => void }) {
  const formats = Object.keys(clip.formats ?? {})
  return <article className="rounded-lg border bg-background p-4 transition-colors hover:bg-muted/20 sm:p-5"><div className="flex items-start justify-between gap-4"><div className="flex min-w-0 items-center gap-3"><div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">{clip.kind === "text" ? <Clipboard className="h-4 w-4" /> : <FileUp className="h-4 w-4" />}</div><div className="min-w-0"><div className="flex flex-wrap items-center gap-2"><Badge variant="outline">{clip.kind === "text" ? "文本" : "文件"}</Badge><span className="truncate text-xs text-muted-foreground">{clip.deviceName || "未知设备"}</span></div><p className="mt-1 text-xs text-muted-foreground">{formatRelativeTime(clip.createdAt)}{clip.expiresAt && ` · ${formatExpiry(clip.expiresAt)}`}</p></div></div><Button variant="ghost" size="icon" className="shrink-0 text-muted-foreground hover:bg-muted hover:text-foreground" onClick={onDelete} disabled={deleting} aria-label="删除记录">{deleting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}</Button></div>{clip.kind === "text" ? <><div className="mt-4 max-h-40 overflow-hidden whitespace-pre-wrap break-words rounded-lg bg-muted/55 p-3 text-sm leading-6">{plainText(clip)}</div><div className="mt-3 flex flex-wrap items-center justify-between gap-2"><div className="flex flex-wrap gap-1.5">{formats.map((format) => <Badge key={format} variant="secondary" className="font-mono text-[10px]">{format}</Badge>)}</div><Button variant="outline" size="sm" onClick={onCopy} disabled={copying}>{copying ? <Loader2 className="h-4 w-4 animate-spin" /> : <Copy className="h-4 w-4" />}复制</Button></div></> : <div className="mt-4 flex items-center gap-3 rounded-lg bg-muted/55 p-3"><FileUp className="h-5 w-5 shrink-0" /><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{clip.name || "未命名文件"}</p><p className="mt-1 truncate text-xs text-muted-foreground">{clip.mimeType || "未知格式"} · {formatBytes(clip.size)}</p></div><Button variant="outline" size="sm" onClick={onDownload}><Download className="h-4 w-4" />下载</Button></div>}</article>
}

function Pagination({ page, pageCount, onPageChange }: { page: number; pageCount: number; onPageChange: (page: number) => void }) {
  if (pageCount <= 1) return null
  return <div className="mt-5 flex items-center justify-between border-t pt-4"><p className="text-xs text-muted-foreground">第 {page} / {pageCount} 页</p><div className="flex gap-2"><Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>上一页</Button><Button variant="outline" size="sm" disabled={page >= pageCount} onClick={() => onPageChange(page + 1)}>下一页</Button></div></div>
}

function AuthOverlay({ username, password, setUsername, setPassword, loading, onLogin }: { username: string; password: string; setUsername: (value: string) => void; setPassword: (value: string) => void; loading: boolean; onLogin: () => void }) {
  return <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"><Card className="w-full max-w-md shadow-2xl"><CardHeader><div className="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-primary text-primary-foreground"><LockKeyhole className="h-5 w-5" /></div><CardTitle>登录 Clipmesh</CardTitle><CardDescription>请输入 NAS 上的账号和密码。每个账号只能看到自己的剪贴板数据。</CardDescription></CardHeader><CardContent className="space-y-3"><div className="space-y-2"><Label htmlFor="login-username">账号</Label><Input id="login-username" autoFocus value={username} onChange={(event) => setUsername(event.target.value)} placeholder="账号" /></div><div className="space-y-2"><Label htmlFor="login-password">密码</Label><Input id="login-password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} onKeyDown={(event) => { if (event.key === "Enter") onLogin() }} placeholder="密码" /></div><Button className="mt-2 w-full" onClick={onLogin} disabled={loading || !username.trim() || !password}><KeyRound className="h-4 w-4" />{loading ? "登录中…" : "登录并开始同步"}</Button></CardContent></Card></div>
}

function LoadingState() {
  return <div className="flex min-h-36 items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground"><Loader2 className="mr-2 h-4 w-4 animate-spin" />正在加载同步记录…</div>
}

function EmptyState({ hasSearch }: { hasSearch: boolean }) {
  return <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border border-dashed px-6 text-center"><div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted text-muted-foreground"><Search className="h-5 w-5" /></div><p className="mt-3 text-sm font-medium">{hasSearch ? "没有匹配的记录" : "还没有同步内容"}</p><p className="mt-1 text-xs text-muted-foreground">{hasSearch ? "换一个关键词或筛选条件试试" : "从左侧发出第一条文本或文件"}</p></div>
}

function plainText(clip: Clip) {
  return clip.formats?.["text/plain"] ?? clip.preview ?? ""
}

function formatExpiry(value: string) {
  return `保留至 ${new Date(value).toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" })}`
}

export default App
