export type HttpMethod = "GET" | "POST" | "PUT" | "DELETE" | "PATCH"

export interface HttpRequest {
  id: string
  timestamp: Date
  method: HttpMethod
  path: string
  statusCode: number
  duration: number
}

export interface RequestDetails {
  request: {
    headers: Record<string, string>
    body?: string
  }
  response: {
    headers: Record<string, string>
    body: string
  }
}

export interface TunnelData {
  url: string
  status: "online" | "offline" | "connecting"
  region: string
  latency: number
  requestsToday: number
}
