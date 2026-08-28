import { useEffect, useMemo, useState, type ReactNode } from "react"
import { Loader2, MonitorSmartphone, Plus, Search } from "lucide-react"

import { ClipRow } from "@/components/clipboard/clip-row"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { filterClips, type ClipFilter } from "@/lib/clips"
import { cn } from "@/lib/utils"
import type { Clip } from "@/types"

interface ClipboardHistoryProps {
  clips: Clip[]
  loading: boolean
  pageSize: number
  copyingId: string
  deletingId: string
  onCopy: (clip: Clip) => void
  onDownload: (clip: Clip) => void
  onDelete: (clip: Clip) => void
  onNew: () => void
}

const filters: Array<{ value: ClipFilter; label: string }> = [
  { value: "all", label: "全部" },
  { value: "text", label: "文本" },
  { value: "file", label: "文件" },
]

export function ClipboardHistory({
  clips,
  loading,
  pageSize,
  copyingId,
  deletingId,
  onCopy,
  onDownload,
  onDelete,
  onNew,
}: ClipboardHistoryProps) {
  const [search, setSearch] = useState("")
  const [filter, setFilter] = useState<ClipFilter>("all")
  const [page, setPage] = useState(1)
  const visibleClips = useMemo(
    () => filterClips(clips, search, filter),
    [clips, filter, search],
  )
  const normalizedPageSize = Math.max(1, pageSize)
  const pageCount = Math.max(1, Math.ceil(visibleClips.length / normalizedPageSize))
  const pageClips = visibleClips.slice(
    (page - 1) * normalizedPageSize,
    page * normalizedPageSize,
  )

  useEffect(() => setPage(1), [filter, search])
  useEffect(() => setPage((current) => Math.min(current, pageCount)), [pageCount])

  return (
    <section className="min-w-0" aria-labelledby="clipboard-history-title">
      <Card className="overflow-hidden border-border/80 shadow-soft">
        <CardHeader className="space-y-4 border-b p-4 sm:p-5">
          <div className="flex items-start justify-between gap-3">
            <div className="flex min-w-0 items-center gap-2">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                <MonitorSmartphone className="h-4 w-4" />
              </div>
              <div className="min-w-0">
                <CardTitle id="clipboard-history-title" className="text-base sm:text-lg">
                  历史剪贴板
                </CardTitle>
                <CardDescription className="mt-1">
                  最新同步内容已置顶，仅当前账号可见
                </CardDescription>
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <Badge variant="secondary">{visibleClips.length} 条</Badge>
              <Button
                variant="outline"
                size="sm"
                className="lg:hidden"
                onClick={onNew}
              >
                <Plus className="h-4 w-4" />
                新建
              </Button>
            </div>
          </div>

          <div className="flex flex-col gap-2 sm:flex-row">
            <div className="relative min-w-0 flex-1">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="搜索内容、设备或文件名"
                className="h-9 bg-background pl-9"
              />
            </div>
            <div className="grid shrink-0 grid-cols-3 rounded-lg border bg-muted/35 p-1">
              {filters.map((item) => (
                <button
                  key={item.value}
                  type="button"
                  onClick={() => setFilter(item.value)}
                  className={cn(
                    "rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                    filter === item.value
                      ? "bg-background text-foreground shadow-sm"
                      : "text-muted-foreground hover:text-foreground",
                  )}
                >
                  {item.label}
                </button>
              ))}
            </div>
          </div>
        </CardHeader>

        <CardContent className="p-0">
          {loading ? (
            <StatePanel>
              <Loader2 className="h-4 w-4 animate-spin" />
              正在加载同步记录…
            </StatePanel>
          ) : pageClips.length === 0 ? (
            <EmptyState filtered={Boolean(search || filter !== "all")} />
          ) : (
            <div className="divide-y">
              {pageClips.map((clip) => (
                <ClipRow
                  key={clip.id}
                  clip={clip}
                  copying={copyingId === clip.id}
                  deleting={deletingId === clip.id}
                  onCopy={() => onCopy(clip)}
                  onDownload={() => onDownload(clip)}
                  onDelete={() => onDelete(clip)}
                />
              ))}
            </div>
          )}
          {pageClips.length > 0 && (
            <Pagination page={page} pageCount={pageCount} onPageChange={setPage} />
          )}
        </CardContent>
      </Card>
    </section>
  )
}

function Pagination({
  page,
  pageCount,
  onPageChange,
}: {
  page: number
  pageCount: number
  onPageChange: (page: number) => void
}) {
  if (pageCount <= 1) return null

  return (
    <div className="flex items-center justify-between border-t bg-muted/15 px-4 py-3 sm:px-5">
      <p className="text-xs text-muted-foreground">
        第 {page} / {pageCount} 页
      </p>
      <div className="flex gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        >
          上一页
        </Button>
        <Button
          variant="outline"
          size="sm"
          disabled={page >= pageCount}
          onClick={() => onPageChange(page + 1)}
        >
          下一页
        </Button>
      </div>
    </div>
  )
}

function StatePanel({ children }: { children: ReactNode }) {
  return (
    <div className="m-4 flex min-h-36 items-center justify-center gap-2 rounded-lg border border-dashed text-sm text-muted-foreground sm:m-5">
      {children}
    </div>
  )
}

function EmptyState({ filtered }: { filtered: boolean }) {
  return (
    <div className="m-4 flex min-h-48 flex-col items-center justify-center rounded-lg border border-dashed px-6 text-center sm:m-5">
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
        <Search className="h-5 w-5" />
      </div>
      <p className="mt-3 text-sm font-medium">
        {filtered ? "没有匹配的记录" : "还没有同步内容"}
      </p>
      <p className="mt-1 text-xs text-muted-foreground">
        {filtered
          ? "换一个关键词或筛选条件试试"
          : "点击“新建”同步第一条文本、图片或文件"}
      </p>
    </div>
  )
}
