import { HttpRequest, RequestDetails } from "./types"

export const mockTunnelData = {
  url: "https://mm8aeh.goport.uz",
  status: "online" as const,
  region: "Tashkent, UZ",
  latency: 12,
  requestsToday: 1847,
}

const mockHeaders = {
  request: {
    'Host': 'mm8aeh.goport.uz',
    'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)',
    'Accept': 'application/json',
    'Accept-Language': 'en-US,en;q=0.9',
    'Accept-Encoding': 'gzip, deflate, br',
    'Connection': 'keep-alive',
    'Cache-Control': 'no-cache',
    'Content-Type': 'application/json',
  },
  response: {
    'Content-Type': 'application/json; charset=utf-8',
    'X-Request-Id': 'req_3x7k9m2n4b',
    'X-Response-Time': '45ms',
    'Cache-Control': 'no-store',
    'Access-Control-Allow-Origin': '*',
    'Server': 'GoPort/1.0',
  }
}

const mockBodies = {
  userGet: JSON.stringify({
    id: "usr_2x8kJ9mN",
    email: "dev@goport.uz",
    name: "Sherzod Karimov",
    plan: "pro",
    tunnels_active: 3
  }, null, 2),
  
  authPost: JSON.stringify({
    access_token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    refresh_token: "rt_8x9kJm2N4b6v...",
    expires_in: 3600,
    token_type: "Bearer"
  }, null, 2),
  
  webhookPost: JSON.stringify({
    event: "tunnel.connected",
    timestamp: "2024-01-15T14:32:00Z",
    data: {
      tunnel_id: "tun_3x7k9m",
      region: "uz-tashkent",
      url: "https://mm8aeh.goport.uz"
    }
  }, null, 2),
  
  configPut: JSON.stringify({
    tunnel_id: "tun_3x7k9m",
    config: {
      timeout: 30000,
      retries: 3,
      rate_limit: 1000
    }
  }, null, 2),
  
  error404: JSON.stringify({
    error: "Not Found",
    message: "The requested resource could not be found",
    code: "RESOURCE_NOT_FOUND"
  }, null, 2),
  
  error500: JSON.stringify({
    error: "Internal Server Error",
    message: "An unexpected error occurred",
    code: "INTERNAL_ERROR",
    request_id: "req_err_9x2m4k"
  }, null, 2),
  
  deleteResponse: JSON.stringify({
    success: true,
    message: "Session terminated successfully",
    session_id: "sess_4x8kN2m"
  }, null, 2),
  
  healthCheck: JSON.stringify({
    status: "healthy",
    version: "1.2.0",
    uptime: 847293,
    services: {
      database: "connected",
      cache: "connected",
      queue: "connected"
    }
  }, null, 2),
}

export const mockRequests: HttpRequest[] = [
  {
    id: "1",
    timestamp: new Date(Date.now() - 1000 * 5),
    method: "GET",
    path: "/api/v1/users/me",
    statusCode: 200,
    duration: 45,
  },
  {
    id: "2",
    timestamp: new Date(Date.now() - 1000 * 12),
    method: "POST",
    path: "/api/v1/auth/refresh",
    statusCode: 200,
    duration: 128,
  },
  {
    id: "3",
    timestamp: new Date(Date.now() - 1000 * 28),
    method: "GET",
    path: "/api/v1/tunnels",
    statusCode: 200,
    duration: 67,
  },
  {
    id: "4",
    timestamp: new Date(Date.now() - 1000 * 45),
    method: "POST",
    path: "/webhooks/stripe",
    statusCode: 200,
    duration: 234,
  },
  {
    id: "5",
    timestamp: new Date(Date.now() - 1000 * 67),
    method: "PUT",
    path: "/api/v1/tunnels/tun_3x7k9m/config",
    statusCode: 200,
    duration: 89,
  },
  {
    id: "6",
    timestamp: new Date(Date.now() - 1000 * 89),
    method: "GET",
    path: "/api/v1/analytics/requests",
    statusCode: 404,
    duration: 23,
  },
  {
    id: "7",
    timestamp: new Date(Date.now() - 1000 * 120),
    method: "DELETE",
    path: "/api/v1/sessions/sess_4x8kN2m",
    statusCode: 200,
    duration: 156,
  },
  {
    id: "8",
    timestamp: new Date(Date.now() - 1000 * 145),
    method: "POST",
    path: "/api/v1/tunnels/create",
    statusCode: 500,
    duration: 2340,
  },
  {
    id: "9",
    timestamp: new Date(Date.now() - 1000 * 180),
    method: "GET",
    path: "/health",
    statusCode: 200,
    duration: 8,
  },
  {
    id: "10",
    timestamp: new Date(Date.now() - 1000 * 210),
    method: "GET",
    path: "/api/v1/billing/invoices",
    statusCode: 401,
    duration: 34,
  },
  {
    id: "11",
    timestamp: new Date(Date.now() - 1000 * 250),
    method: "POST",
    path: "/api/v1/auth/login",
    statusCode: 200,
    duration: 312,
  },
  {
    id: "12",
    timestamp: new Date(Date.now() - 1000 * 300),
    method: "GET",
    path: "/api/v1/users/preferences",
    statusCode: 200,
    duration: 56,
  },
]

export function getRequestDetails(request: HttpRequest): RequestDetails {
  let requestBody = ""
  let responseBody = ""
  
  if (request.method === "GET" && request.path.includes("/users/me")) {
    responseBody = mockBodies.userGet
  } else if (request.method === "POST" && request.path.includes("/auth")) {
    requestBody = JSON.stringify({ email: "dev@goport.uz", password: "••••••••" }, null, 2)
    responseBody = mockBodies.authPost
  } else if (request.path.includes("/webhooks")) {
    requestBody = mockBodies.webhookPost
    responseBody = JSON.stringify({ received: true }, null, 2)
  } else if (request.method === "PUT") {
    requestBody = mockBodies.configPut
    responseBody = JSON.stringify({ updated: true, tunnel_id: "tun_3x7k9m" }, null, 2)
  } else if (request.statusCode === 404) {
    responseBody = mockBodies.error404
  } else if (request.statusCode === 500) {
    requestBody = JSON.stringify({ name: "new-tunnel", region: "uz-tashkent" }, null, 2)
    responseBody = mockBodies.error500
  } else if (request.method === "DELETE") {
    responseBody = mockBodies.deleteResponse
  } else if (request.path.includes("/health")) {
    responseBody = mockBodies.healthCheck
  } else if (request.statusCode === 401) {
    responseBody = JSON.stringify({ error: "Unauthorized", message: "Invalid or expired token" }, null, 2)
  } else {
    responseBody = JSON.stringify({ success: true, data: [] }, null, 2)
  }

  return {
    request: {
      headers: mockHeaders.request,
      body: requestBody || undefined,
    },
    response: {
      headers: {
        ...mockHeaders.response,
        'X-Response-Time': `${request.duration}ms`,
      },
      body: responseBody,
    }
  }
}
