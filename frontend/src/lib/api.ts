import type { AppConfig, Clip, ClipEvent, User } from "@/types"
import { getDeviceId } from "@/lib/device"

const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/$/, "")

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = "ApiError"
    this.status = status
  }
}

function url(path: string) {
  return `${API_BASE}${path}`
}

async function request<T>(path: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers)
  headers.set("Accept", "application/json")
  if (init.body && !(init.body instanceof FormData)) headers.set("Content-Type", "application/json")

  const response = await fetch(url(path), { ...init, headers, credentials: "include" })
  if (!response.ok) {
    let message = response.statusText
    try {
      const body = (await response.json()) as { error?: string }
      message = body.error ?? message
    } catch {
      // Keep the HTTP status text when the server did not return JSON.
    }
    throw new ApiError(response.status, message)
  }
  if (response.status === 204) return undefined as T
  return (await response.json()) as T
}

export function fetchConfig() {
  return request<AppConfig>("/api/config")
}

export function login(username: string, password: string) {
  return request<User>("/api/auth/login", { method: "POST", body: JSON.stringify({ username, password }) })
}

export function logout() {
  return request<void>("/api/auth/logout", { method: "POST" })
}

export function fetchCurrentUser() {
  return request<User>("/api/auth/me")
}

export async function fetchClips() {
  const result = await request<{ items: Clip[] }>("/api/clips?limit=200")
  return result.items
}

export function createText(formats: Record<string, string>, deviceName: string) {
  return request<Clip>("/api/clips", {
    method: "POST",
    body: JSON.stringify({ kind: "text", deviceId: getDeviceId(), deviceName, formats }),
  })
}

export function uploadFile(file: File, deviceName: string) {
  const body = new FormData()
  body.append("file", file)
  body.append("deviceId", getDeviceId())
  body.append("deviceName", deviceName)
  body.append("formats", JSON.stringify({ file: file.type || "application/octet-stream" }))
  return request<Clip>("/api/clips/file", { method: "POST", body })
}

export async function deleteClip(id: string) {
  await request<void>(`/api/clips/${encodeURIComponent(id)}`, { method: "DELETE" })
}

export async function downloadFile(id: string) {
  const response = await fetch(url(`/api/clips/${encodeURIComponent(id)}/file`), { credentials: "include" })
  if (!response.ok) throw new ApiError(response.status, "文件下载失败")
  return response.blob()
}

export async function fetchUsers() {
  const result = await request<{ items: User[] }>("/api/admin/users")
  return result.items
}

export function createUser(username: string, password: string, role: User["role"] = "user") {
  return request<User>("/api/admin/users", { method: "POST", body: JSON.stringify({ username, password, role }) })
}

export async function deleteUser(id: string) {
  await request<void>(`/api/admin/users/${encodeURIComponent(id)}`, { method: "DELETE" })
}

export interface EventConnection {
  onopen: (() => void) | null
  onerror: (() => void) | null
  onreconnecting: (() => void) | null
  close: () => void
}

export function connectEvents(onEvent: (event: ClipEvent) => void): EventConnection {
  let active = true
  let controller: AbortController | undefined
  const connection: EventConnection = {
    onopen: null,
    onerror: null,
    onreconnecting: null,
    close: () => {
      active = false
      controller?.abort()
    },
  }
  const headers = new Headers({ Accept: "text/event-stream" })
  let lastEventId = ""

  void (async () => {
    let retryDelay = 1000
    while (active) {
      controller = new AbortController()
      try {
        const requestHeaders = new Headers(headers)
        if (lastEventId) requestHeaders.set("Last-Event-ID", lastEventId)
        const response = await fetch(url("/api/events"), { headers: requestHeaders, signal: controller.signal, credentials: "include" })
        if (!response.ok || !response.body) throw new Error("SSE connection failed")
        connection.onopen?.()
        retryDelay = 1000
        const reader = response.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ""
        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true })
          const messages = buffer.split(/\r?\n\r?\n/)
          buffer = messages.pop() ?? ""
          for (const message of messages) {
            let eventName = "message"
            let data = ""
            let eventId = ""
            for (const line of message.split(/\r?\n/)) {
              if (line.startsWith("event:")) eventName = line.slice(6).trim()
              if (line.startsWith("data:")) data += line.slice(5).trim()
              if (line.startsWith("id:")) eventId = line.slice(3).trim()
            }
            if (eventId) lastEventId = eventId
            if (eventName === "clip" && data) {
              try {
                onEvent(JSON.parse(data) as ClipEvent)
              } catch {
                // The next refresh reconciles malformed or partial events.
              }
            }
          }
        }
        if (active) connection.onerror?.()
      } catch {
        if (!active) break
        connection.onerror?.()
      }
      if (active) {
        connection.onreconnecting?.()
        await new Promise((resolve) => window.setTimeout(resolve, retryDelay))
        retryDelay = Math.min(retryDelay * 2, 15_000)
      }
    }
  })()
  return connection
}
