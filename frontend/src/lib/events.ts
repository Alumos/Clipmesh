import type { ClipEvent } from "@/types"

const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/$/, "")

export interface EventConnection {
  onopen: (() => void) | null
  onerror: (() => void) | null
  onreconnecting: (() => void) | null
  close: () => void
}

export function connectEvents(onEvent: (event: ClipEvent) => void): EventConnection {
  let active = true
  let controller: AbortController | undefined
  let lastEventId = ""
  const connection: EventConnection = {
    onopen: null,
    onerror: null,
    onreconnecting: null,
    close: () => {
      active = false
      controller?.abort()
    },
  }

  void (async () => {
    let retryDelay = 1000
    while (active) {
      controller = new AbortController()
      try {
        const headers = new Headers({ Accept: "text/event-stream" })
        if (lastEventId) headers.set("Last-Event-ID", lastEventId)
        const response = await fetch(`${API_BASE}/api/events`, {
          headers,
          signal: controller.signal,
          credentials: "include",
        })
        if (!response.ok || !response.body) {
          throw new Error("SSE connection failed")
        }

        connection.onopen?.()
        retryDelay = 1000
        const reader = response.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ""
        while (active) {
          const { done, value } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true })
          const messages = buffer.split(/\r?\n\r?\n/)
          buffer = messages.pop() ?? ""
          for (const message of messages) {
            const parsed = parseMessage(message)
            if (parsed.id) lastEventId = parsed.id
            if (parsed.event === "clip" && parsed.data) {
              try {
                onEvent(JSON.parse(parsed.data) as ClipEvent)
              } catch {
                // The reconnect refresh reconciles malformed or partial events.
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

function parseMessage(message: string) {
  let event = "message"
  let data = ""
  let id = ""
  for (const line of message.split(/\r?\n/)) {
    if (line.startsWith("event:")) event = line.slice(6).trim()
    if (line.startsWith("data:")) data += line.slice(5).trim()
    if (line.startsWith("id:")) id = line.slice(3).trim()
  }
  return { event, data, id }
}
