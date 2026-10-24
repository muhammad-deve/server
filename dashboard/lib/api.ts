import { HttpRequest, RequestDetails, TunnelData, HttpMethod } from "./types"

// The inspector API is always same-origin: each CLI serves this UI on its own
// port (4040-4139), and it refuses requests from any other origin.
function apiBase(): string {
  return ""
}

// Required by the CLI on requests that change something. Another site can't
// send it without a CORS preflight, which the CLI never allows.
const WRITE_HEADERS = { "X-GoPort-Inspector": "1" }

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
  plan?: {
    known: boolean
    plan: string
    isPro: boolean
    monthlyBytes: number
    monthBytes: number
  }
  url: string
  status: string
  region: string
  latency: number
  requestsToday: number
  totalBytes: number
  version: string
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
    totalBytes: raw.totalBytes || 0,
    version: raw.version || "",
    plan: raw.plan?.known ? raw.plan : undefined,
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
  const res = await fetch(`${apiBase()}/api/requests/${encodeURIComponent(id)}`)
  if (!res.ok) throw new Error(`details fetch failed: ${res.status}`)
  const raw: RawCapturedRequest = await res.json()
  const details = toRequestDetails(raw)
  detailsCache.set(id, details)
  return details
}

export async function clearRequests(): Promise<void> {
  await fetch(`${apiBase()}/api/requests`, { method: "DELETE", headers: WRITE_HEADERS })
  detailsCache.clear()
}

export async function replayRequest(id: string): Promise<HttpRequest> {
  const res = await fetch(`${apiBase()}/api/requests/${encodeURIComponent(id)}/replay`, {
    method: "POST",
    headers: WRITE_HEADERS,
    // The target is localhost, so 15s is already generous. Without a signal a
    // local app that accepts the socket and never answers hangs the UI forever.
    signal: AbortSignal.timeout(15_000),
  })
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
