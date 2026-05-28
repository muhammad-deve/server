import { HttpRequest, RequestDetails, TunnelData, HttpMethod } from "./types"

// API client talks to the embedded Go server at the same origin the dashboard is served from.
// In dev (next dev) this falls back to localhost:4040 so you can iterate on the UI.
function apiBase(): string {
  if (typeof window === "undefined") return ""
  if (window.location.port === "4040" || window.location.port.startsWith("404")) {
    return ""
  }
  // Dev mode (e.g. next dev on 3000): hit the running goport dashboard.
  return "http://127.0.0.1:4040"
}

interface RawCapturedRequest {
  id: string
  timestamp: string
  method: string
  path: string
  statusCode: number
  duration: number
  reqHeaders: Record<string, string>
  reqBody: string
  resHeaders: Record<string, string>
  resBody: string
}

interface RawTunnelInfo {
  url: string
  status: string
  region: string
  latency: number
  requestsToday: number
  started: string
}

function toHttpRequest(r: RawCapturedRequest): HttpRequest {
  return {
    id: r.id,
    timestamp: new Date(r.timestamp),
    method: r.method as HttpMethod,
    path: r.path,
    statusCode: r.statusCode,
    duration: r.duration,
  }
}

function toRequestDetails(r: RawCapturedRequest): RequestDetails {
  return {
    request: {
      headers: r.reqHeaders || {},
      body: r.reqBody || undefined,
    },
    response: {
      headers: r.resHeaders || {},
      body: r.resBody || "",
    },
  }
}

const detailsCache = new Map<string, RequestDetails>()

export async function fetchTunnel(): Promise<TunnelData> {
  const res = await fetch(`${apiBase()}/api/tunnel`)
  if (!res.ok) throw new Error(`tunnel fetch failed: ${res.status}`)
  const raw: RawTunnelInfo = await res.json()
  return {
    url: raw.url || "",
    status: (raw.status || "online") as TunnelData["status"],
    region: raw.region || "",
    latency: raw.latency || 0,
    requestsToday: raw.requestsToday || 0,
  }
}

export async function fetchRequests(): Promise<HttpRequest[]> {
  const res = await fetch(`${apiBase()}/api/requests`)
  if (!res.ok) throw new Error(`requests fetch failed: ${res.status}`)
  const raw: RawCapturedRequest[] = (await res.json()) || []
  for (const r of raw) detailsCache.set(r.id, toRequestDetails(r))
  return raw.map(toHttpRequest)
}

export async function fetchRequestDetails(id: string): Promise<RequestDetails> {
  const cached = detailsCache.get(id)
  if (cached) return cached
  const res = await fetch(`${apiBase()}/api/requests/${id}`)
  if (!res.ok) throw new Error(`details fetch failed: ${res.status}`)
  const raw: RawCapturedRequest = await res.json()
  const details = toRequestDetails(raw)
  detailsCache.set(id, details)
  return details
}

export async function clearRequests(): Promise<void> {
  await fetch(`${apiBase()}/api/requests`, { method: "DELETE" })
  detailsCache.clear()
}

export async function replayRequest(id: string): Promise<HttpRequest> {
  const res = await fetch(`${apiBase()}/api/requests/${id}/replay`, { method: "POST" })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `replay failed: ${res.status}`)
  }
  const raw: RawCapturedRequest = await res.json()
  detailsCache.set(raw.id, toRequestDetails(raw))
  return toHttpRequest(raw)
}

// subscribeRequests opens an SSE stream that pushes new captured requests as
// they're tunneled. Returns a teardown function.
export function subscribeRequests(onRequest: (r: HttpRequest, details: RequestDetails) => void): () => void {
  const url = `${apiBase()}/api/events`
  const es = new EventSource(url)
  es.addEventListener("request", (event) => {
    try {
      const raw: RawCapturedRequest = JSON.parse((event as MessageEvent).data)
      const details = toRequestDetails(raw)
      detailsCache.set(raw.id, details)
      onRequest(toHttpRequest(raw), details)
    } catch {
      // ignore malformed events
    }
  })
  return () => es.close()
}
