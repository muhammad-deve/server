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

/** Account allowance, supplied by the GoPort API via the CLI. */
export interface PlanInfo {
  /** False until the API has answered -- offline, signed out, or self-hosted. */
  known: boolean
  plan: string
  isPro: boolean
  /** Traffic included per month. */
  monthlyBytes: number
  /** Traffic used so far this month, measured the same way the limit is enforced. */
  monthBytes: number
}

export interface TunnelData {
  url: string
  status: "online" | "offline" | "connecting"
  region: string
  latency: number
  requestsToday: number
  totalBytes: number
  version: string
  plan?: PlanInfo
}
