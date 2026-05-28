"use client"

import { useState, useRef, useEffect } from "react"
import { ChevronRight, Clock, RotateCw } from "lucide-react"
import { HttpRequest, RequestDetails } from "@/lib/types"
import { MethodBadge, StatusBadge } from "./badges"
import { CodeBlock, HeadersBlock, CurlBlock } from "./code-block"
import { fetchRequestDetails } from "@/lib/api"

interface RequestRowProps {
  request: HttpRequest
  onReplay: (request: HttpRequest) => void
  tunnelUrl: string
}

function generateCurlCommand(request: HttpRequest, details: RequestDetails, tunnelUrl: string): string {
  const baseUrl = tunnelUrl || ""
  const lines: string[] = []

  if (request.method === "GET") {
    lines.push(`curl ${baseUrl}${request.path}`)
  } else {
    lines.push(`curl -X ${request.method} ${baseUrl}${request.path}`)
  }

  const relevantHeaders = ["Content-Type", "Authorization", "Accept"]
  for (const header of relevantHeaders) {
    if (details.request.headers[header]) {
      lines.push(`  -H "${header}: ${details.request.headers[header]}"`)
    }
  }

  if (details.request.body) {
    const trimmed = details.request.body.trim()
    if (trimmed) {
      // Escape single quotes for shell.
      const escaped = trimmed.replace(/'/g, `'\\''`)
      lines.push(`  -d '${escaped}'`)
    }
  }

  return lines.join(" \\\n")
}

export function RequestRow({ request, onReplay, tunnelUrl }: RequestRowProps) {
  const [isExpanded, setIsExpanded] = useState(false)
  const [activeTab, setActiveTab] = useState<"request" | "response" | "curl">("request")
  const [details, setDetails] = useState<RequestDetails | null>(null)
  const [detailsError, setDetailsError] = useState<string | null>(null)
  const [isReplaying, setIsReplaying] = useState(false)
  const [formattedTime, setFormattedTime] = useState<string>("")
  const replayButtonRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    setFormattedTime(
      request.timestamp.toLocaleTimeString("en-US", {
        hour12: false,
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      })
    )
  }, [request.timestamp])

  const handleExpand = async () => {
    if (!isExpanded && !details && !detailsError) {
      try {
        const d = await fetchRequestDetails(request.id)
        setDetails(d)
      } catch (err) {
        setDetailsError(err instanceof Error ? err.message : "failed to load details")
      }
    }
    setIsExpanded(!isExpanded)
  }

  const handleReplay = (e: React.MouseEvent) => {
    e.stopPropagation()
    setIsReplaying(true)

    if (replayButtonRef.current) {
      replayButtonRef.current.classList.add("replay-spin")
    }

    // Run replay through the API; the spinner clears once it returns.
    Promise.resolve(onReplay(request)).finally(() => {
      setIsReplaying(false)
      if (replayButtonRef.current) {
        replayButtonRef.current.classList.remove("replay-spin")
      }
    })
  }

  return (
    <div className="group">
      <div
        onClick={handleExpand}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault()
            handleExpand()
          }
        }}
        className={`w-full px-4 py-2.5 flex items-center gap-4 transition-colors duration-150 cursor-pointer hover:bg-[var(--goport-bg-tertiary)] ${
          isExpanded ? "bg-[var(--goport-bg-tertiary)]" : ""
        }`}
      >
        <ChevronRight
          className={`w-3.5 h-3.5 text-[var(--goport-text-muted)] transition-transform duration-150 ${
            isExpanded ? "rotate-90" : ""
          }`}
        />

        <div className="flex items-center gap-1.5 w-20 shrink-0">
          <Clock className="w-3 h-3 text-[var(--goport-text-muted)]" />
          <span className="text-xs text-[var(--goport-text-muted)] font-mono">
            {formattedTime || "--:--:--"}
          </span>
        </div>

        <div className="w-16 shrink-0">
          <MethodBadge method={request.method} />
        </div>

        <div className="flex-1 text-left truncate">
          <code className="text-sm text-[var(--goport-text-secondary)] font-mono">{request.path}</code>
        </div>

        <div className="w-14 shrink-0">
          <StatusBadge statusCode={request.statusCode} />
        </div>

        <div className="w-16 shrink-0 text-right">
          <span
            className={`text-xs font-mono ${
              request.duration > 1000
                ? "text-[var(--goport-error)]"
                : request.duration > 200
                ? "text-[var(--goport-warning)]"
                : "text-[var(--goport-text-muted)]"
            }`}
          >
            {request.duration}ms
          </span>
        </div>

        <button
          ref={replayButtonRef}
          onClick={handleReplay}
          disabled={isReplaying}
          className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 transition-opacity hover:bg-[var(--goport-border)] disabled:opacity-50"
          title="Replay request"
          aria-label="Replay request"
        >
          <RotateCw className="w-3.5 h-3.5 text-[var(--goport-text-muted)]" />
        </button>
      </div>

      <div
        className={`overflow-hidden transition-all duration-200 ease-out ${
          isExpanded ? "max-h-[600px] opacity-100" : "max-h-0 opacity-0"
        }`}
      >
        {detailsError ? (
          <div className="px-4 pb-4 pt-2 bg-[var(--goport-bg-tertiary)] border-t border-[var(--goport-border)]/50">
            <div className="text-xs text-[var(--goport-error)]">Failed to load details: {detailsError}</div>
          </div>
        ) : details ? (
          <div className="px-4 pb-4 pt-2 bg-[var(--goport-bg-tertiary)] border-t border-[var(--goport-border)]/50">
            <div className="flex gap-0.5 mb-4 p-0.5 bg-[var(--goport-bg)] rounded-md w-fit border border-[var(--goport-border)]">
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  setActiveTab("request")
                }}
                className={`px-3 py-1.5 rounded text-xs font-medium transition-colors ${
                  activeTab === "request"
                    ? "bg-[var(--goport-bg-tertiary)] text-[var(--goport-text-secondary)]"
                    : "text-[var(--goport-text-muted)] hover:text-[var(--goport-text-secondary)]"
                }`}
              >
                Request
              </button>
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  setActiveTab("response")
                }}
                className={`px-3 py-1.5 rounded text-xs font-medium transition-colors ${
                  activeTab === "response"
                    ? "bg-[var(--goport-bg-tertiary)] text-[var(--goport-text-secondary)]"
                    : "text-[var(--goport-text-muted)] hover:text-[var(--goport-text-secondary)]"
                }`}
              >
                Response
              </button>
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  setActiveTab("curl")
                }}
                className={`px-3 py-1.5 rounded text-xs font-medium transition-colors ${
                  activeTab === "curl"
                    ? "bg-[var(--goport-bg-tertiary)] text-[var(--goport-text-secondary)]"
                    : "text-[var(--goport-text-muted)] hover:text-[var(--goport-text-secondary)]"
                }`}
              >
                curl
              </button>
            </div>

            <div className="space-y-3" onClick={(e) => e.stopPropagation()}>
              {activeTab === "request" ? (
                <>
                  <HeadersBlock headers={details.request.headers} />
                  <CodeBlock title="Body" content={details.request.body || ""} />
                </>
              ) : activeTab === "response" ? (
                <>
                  <HeadersBlock headers={details.response.headers} />
                  <CodeBlock title="Body" content={details.response.body} />
                </>
              ) : (
                <CurlBlock curl={generateCurlCommand(request, details, tunnelUrl)} />
              )}
            </div>
          </div>
        ) : (
          <div className="px-4 pb-4 pt-2 bg-[var(--goport-bg-tertiary)] border-t border-[var(--goport-border)]/50">
            <div className="text-xs text-[var(--goport-text-muted)]">Loading details…</div>
          </div>
        )}
      </div>
    </div>
  )
}

interface RequestsTableProps {
  requests: HttpRequest[]
  onReplay: (request: HttpRequest) => void
  tunnelUrl?: string
}

export function RequestsTable({ requests, onReplay, tunnelUrl = "" }: RequestsTableProps) {
  if (requests.length === 0) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center text-center p-8">
        <div className="w-12 h-12 rounded-full bg-[var(--goport-bg-tertiary)] border border-[var(--goport-border)] flex items-center justify-center mb-4">
          <div className="w-2 h-2 rounded-full bg-[var(--goport-success)] animate-pulse-dot"></div>
        </div>
        <h3 className="text-sm font-medium text-[var(--goport-text-secondary)] mb-1">Waiting for requests</h3>
        <p className="text-xs text-[var(--goport-text-muted)] max-w-xs">
          Send HTTP requests to your tunnel URL and they will appear here.
        </p>
      </div>
    )
  }

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="sticky top-0 z-10 px-4 py-2.5 bg-[var(--goport-bg-secondary)] border-b border-[var(--goport-border)] flex items-center gap-4">
        <div className="w-3.5"></div>
        <div className="w-20 shrink-0 text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">Time</div>
        <div className="w-16 shrink-0 text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">Method</div>
        <div className="flex-1 text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">Path</div>
        <div className="w-14 shrink-0 text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">Status</div>
        <div className="w-16 shrink-0 text-right text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">Duration</div>
        <div className="w-8"></div>
      </div>

      <div className="divide-y divide-[var(--goport-border)]/50">
        {requests.map((request) => (
          <RequestRow key={request.id} request={request} onReplay={onReplay} tunnelUrl={tunnelUrl} />
        ))}
      </div>
    </div>
  )
}
