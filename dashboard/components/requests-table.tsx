"use client"

import { useEffect, useId, useRef, useState } from "react"
import { Check, ChevronRight, Radio, RotateCw, SearchX } from "lucide-react"
import { HttpRequest, RequestDetails } from "@/lib/types"
import { MethodBadge, StatusBadge } from "./badges"
import { CodeBlock, HeadersBlock, CurlBlock } from "./code-block"
import { CopyButton } from "./copy-button"
import { fetchRequestDetails } from "@/lib/api"

type Tab = "request" | "response" | "curl"

const TABS: { id: Tab; label: string }[] = [
  { id: "request", label: "Request" },
  { id: "response", label: "Response" },
  { id: "curl", label: "cURL" },
]

/**
 * POSIX single-quoting. Header values and bodies are attacker-controlled, and
 * the result is a command the developer pastes into their own shell -- double
 * quotes would let `$(...)` in a header execute on their machine.
 */
const shellQuote = (s: string) => `'${s.replace(/'/g, `'\\''`)}'`

function generateCurlCommand(request: HttpRequest, details: RequestDetails, tunnelUrl: string): string {
  const url = `${tunnelUrl || ""}${request.path}`
  const lines: string[] = [
    request.method === "GET" ? `curl ${shellQuote(url)}` : `curl -X ${request.method} ${shellQuote(url)}`,
  ]

  for (const header of ["Content-Type", "Authorization", "Accept"]) {
    const value = details.request.headers[header]
    if (value) lines.push(`  -H ${shellQuote(`${header}: ${value}`)}`)
  }

  const body = details.request.body?.trim()
  if (body) lines.push(`  -d ${shellQuote(body)}`)

  return lines.join(" \\\n")
}

/*
 * Replay feedback timing. A replay to localhost usually lands in under 100ms,
 * so a spinner shown immediately appears and vanishes before it can be read --
 * that registers as a glitch, not as feedback. Instead: stay silent while the
 * work is near-instant, confirm with a tick, and only fall back to a spinner
 * when the wait is actually long enough to need one.
 */
const SPINNER_DELAY_MS = 180   // below this, never show a spinner at all
const SPINNER_MIN_MS = 420     // once shown, keep it long enough to read
const DONE_HOLD_MS = 1400      // how long the success tick lingers
const REPLAY_TIMEOUT_MS = 20_000

type ReplayState = "idle" | "busy" | "done" | "error"

function durationColor(ms: number) {
  if (ms > 1000) return "var(--status-5xx)"
  if (ms > 300) return "var(--status-4xx)"
  return "var(--muted-foreground)"
}

/** Column widths shared by the header and every row so the grid lines up. */
const COL = {
  chevron: "w-4 shrink-0",
  time: "hidden w-[4.5rem] shrink-0 sm:block",
  method: "w-[4.5rem] shrink-0",
  path: "min-w-0 flex-1",
  status: "w-[4.5rem] shrink-0",
  duration: "hidden w-20 shrink-0 text-right sm:block",
  action: "w-8 shrink-0",
}

/**
 * Proper ARIA tabs: arrow keys move between tabs, only the active tab is in the
 * tab order, and each tab points at its panel. Declaring role="tab" without
 * this makes screen readers promise keyboard behaviour that does not exist.
 */
function DetailTabs({ active, onChange, baseId }: { active: Tab; onChange: (t: Tab) => void; baseId: string }) {
  const refs = useRef<(HTMLButtonElement | null)[]>([])

  const onKeyDown = (e: React.KeyboardEvent, index: number) => {
    const delta = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : e.key === "Home" ? -index : e.key === "End" ? TABS.length - 1 - index : 0
    if (delta === 0) return
    e.preventDefault()
    const next = (index + delta + TABS.length) % TABS.length
    onChange(TABS[next].id)
    refs.current[next]?.focus()
  }

  return (
    <div role="tablist" aria-label="Request details" className="inline-flex rounded-xl border border-border bg-secondary/60 p-1">
      {TABS.map((tab, index) => (
        <button
          key={tab.id}
          ref={(el) => { refs.current[index] = el }}
          type="button"
          role="tab"
          id={`${baseId}-tab-${tab.id}`}
          aria-selected={active === tab.id}
          aria-controls={`${baseId}-panel`}
          tabIndex={active === tab.id ? 0 : -1}
          onKeyDown={(e) => onKeyDown(e, index)}
          onClick={() => onChange(tab.id)}
          className={`h-8 rounded-lg px-3.5 text-xs font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
            active === tab.id ? "bg-card text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground"
          }`}
        >
          {tab.label}
        </button>
      ))}
    </div>
  )
}

interface RequestRowProps {
  request: HttpRequest
  onReplay: (request: HttpRequest) => void | Promise<void>
  tunnelUrl: string
}

export function RequestRow({ request, onReplay, tunnelUrl }: RequestRowProps) {
  const [isExpanded, setIsExpanded] = useState(false)
  const [activeTab, setActiveTab] = useState<Tab>("request")
  const [details, setDetails] = useState<RequestDetails | null>(null)
  const [detailsError, setDetailsError] = useState<string | null>(null)
  const [replayState, setReplayState] = useState<ReplayState>("idle")
  const [spinnerVisible, setSpinnerVisible] = useState(false)
  const [replayError, setReplayError] = useState<string | null>(null)
  const timers = useRef<ReturnType<typeof setTimeout>[]>([])
  const [formattedTime, setFormattedTime] = useState("")
  const [isFresh] = useState(() => Date.now() - request.timestamp.getTime() < 3000)
  const baseId = useId()

  useEffect(() => () => { timers.current.forEach(clearTimeout) }, [])

  useEffect(() => {
    setFormattedTime(
      request.timestamp.toLocaleTimeString("en-GB", { hour12: false, hour: "2-digit", minute: "2-digit", second: "2-digit" }),
    )
  }, [request.timestamp])

  const toggle = async () => {
    const next = !isExpanded
    setIsExpanded(next)
    if (next && !details) {
      setDetailsError(null)
      try {
        setDetails(await fetchRequestDetails(request.id))
      } catch (err) {
        setDetailsError(err instanceof Error ? err.message : "failed to load details")
      }
    }
  }

  const track = (t: ReturnType<typeof setTimeout>) => {
    timers.current.push(t)
    return t
  }
  const clearTimers = () => {
    timers.current.forEach(clearTimeout)
    timers.current = []
  }

  const handleReplay = async () => {
    if (replayState === "busy") return
    clearTimers()
    setReplayState("busy")
    setReplayError(null)
    setSpinnerVisible(false)

    // The spinner only appears if the work outlasts SPINNER_DELAY_MS.
    let shownAt = 0
    track(setTimeout(() => {
      shownAt = Date.now()
      setSpinnerVisible(true)
    }, SPINNER_DELAY_MS))

    // Watchdog: a wedged request must never leave a control stuck looking busy.
    track(setTimeout(() => {
      clearTimers()
      setSpinnerVisible(false)
      setReplayState("error")
      setReplayError("timed out waiting for the replay to finish")
    }, REPLAY_TIMEOUT_MS))

    let failure: string | null = null
    try {
      await onReplay(request)
    } catch (err) {
      console.error("replay failed", err)
      failure = err instanceof Error ? err.message : "replay failed"
    }

    // If the spinner did become visible, let it sit long enough to be read
    // rather than blinking out the moment the response lands.
    const hold = shownAt ? Math.max(0, SPINNER_MIN_MS - (Date.now() - shownAt)) : 0
    clearTimers()
    track(setTimeout(() => {
      setSpinnerVisible(false)
      setReplayError(failure)
      setReplayState(failure ? "error" : "done")
      if (!failure) track(setTimeout(() => setReplayState("idle"), DONE_HOLD_MS))
    }, hold))
  }

  return (
    <div className={isFresh ? "animate-row-in" : undefined}>
      <span className="sr-only" role="status">
        {replayState === "done" ? `Replayed ${request.method} ${request.path}` : ""}
        {replayState === "error" && replayError ? `Replay failed: ${replayError}` : ""}
      </span>
      <div
        className={`relative flex items-center gap-3 px-4 transition-colors hover:bg-secondary/60 sm:gap-4 sm:px-6 ${
          isExpanded ? "bg-secondary/60" : ""
        }`}
      >
        {isExpanded ? <span className="absolute inset-y-0 left-0 w-0.5 bg-primary" aria-hidden /> : null}

        {/* The disclosure is a real button. The replay control is a sibling, not
            a descendant -- a button may not contain another focusable element. */}
        <button
          type="button"
          onClick={toggle}
          aria-expanded={isExpanded}
          aria-controls={`${baseId}-panel`}
          className="flex min-w-0 flex-1 items-center gap-3 py-3 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring sm:gap-4"
        >
          <ChevronRight
            className={`${COL.chevron} size-4 text-muted-foreground transition-transform duration-200 ${isExpanded ? "rotate-90 text-foreground" : ""}`}
            aria-hidden
          />
          <span className={`${COL.time} font-mono text-xs tabular-nums text-muted-foreground`}>
            {formattedTime || "--:--:--"}
          </span>
          <span className={COL.method}>
            <MethodBadge method={request.method} />
          </span>
          <code className={`${COL.path} truncate font-mono text-sm text-foreground`} title={request.path}>
            {request.path}
          </code>
          <span className={COL.status}>
            <StatusBadge statusCode={request.statusCode} />
          </span>
          <span className={`${COL.duration} font-mono text-xs tabular-nums`} style={{ color: durationColor(request.duration) }}>
            {request.duration}ms
          </span>
        </button>

        {/* Always visible: a hover-only affordance is unreachable on touch. */}
        <button
          type="button"
          onClick={handleReplay}
          disabled={replayState === "busy"}
          aria-busy={replayState === "busy"}
          title={
            replayState === "error" && replayError
              ? `Replay failed: ${replayError}`
              : replayState === "done"
              ? "Replayed"
              : "Replay request"
          }
          aria-label={`Replay ${request.method} ${request.path}`}
          className={`${COL.action} inline-flex size-8 items-center justify-center rounded-lg transition-colors hover:bg-primary/12 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-100 ${
            replayState === "error" ? "text-destructive" : replayState === "done" ? "text-primary" : "text-muted-foreground"
          }`}
        >
          {/* A fast replay shows no spinner at all -- just the tick. */}
          {replayState === "done" ? (
            <Check className="size-4" aria-hidden />
          ) : (
            <RotateCw className={`size-4 ${spinnerVisible ? "replay-spin text-primary" : ""}`} aria-hidden />
          )}
        </button>
      </div>

      {/* 0fr -> 1fr grows to the real content height. The old max-h-[600px]
          silently cut off anything taller. */}
      <div
        className={`grid transition-[grid-template-rows] duration-200 ease-out ${isExpanded ? "grid-rows-[1fr]" : "grid-rows-[0fr]"}`}
      >
        <div className="overflow-hidden">
          <div
            id={`${baseId}-panel`}
            role={details ? "tabpanel" : undefined}
            aria-labelledby={details ? `${baseId}-tab-${activeTab}` : undefined}
            tabIndex={details ? 0 : undefined}
            className="border-t border-border bg-secondary/25 px-4 pb-5 pt-4 focus-visible:outline-none sm:px-6"
          >
            {detailsError ? (
              <p className="text-xs text-destructive">Could not load details: {detailsError}</p>
            ) : details ? (
              <>
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <DetailTabs active={activeTab} onChange={setActiveTab} baseId={baseId} />
                  <div className="flex min-w-0 items-center gap-1">
                    <code className="min-w-0 truncate font-mono text-xs text-muted-foreground" title={`${tunnelUrl}${request.path}`}>
                      {tunnelUrl}{request.path}
                    </code>
                    <CopyButton text={`${tunnelUrl}${request.path}`} className="shrink-0" />
                  </div>
                </div>
                <div className="mt-4 space-y-3">
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
              </>
            ) : (
              <p className="text-xs text-muted-foreground">Loading details...</p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

interface RequestsTableProps {
  requests: HttpRequest[]
  onReplay: (request: HttpRequest) => void
  tunnelUrl?: string
  /** True when a filter is active, so the empty state can say so. */
  filtering?: boolean
}

export function RequestsTable({ requests, onReplay, tunnelUrl = "", filtering = false }: RequestsTableProps) {
  if (requests.length === 0) {
    const Icon = filtering ? SearchX : Radio
    return (
      <div className="flex flex-1 flex-col items-center justify-center p-8 text-center">
        <span className="flex size-12 items-center justify-center rounded-2xl border border-border bg-card text-primary">
          <Icon className="size-5" aria-hidden />
        </span>
        <h3 className="mt-4 text-sm font-semibold text-foreground">
          {filtering ? "No requests match that filter" : "Waiting for requests"}
        </h3>
        <p className="mt-1.5 max-w-sm text-xs leading-5 text-muted-foreground">
          {filtering
            ? "Clear the filter to see everything captured on this tunnel."
            : "Send an HTTP request to your tunnel URL and it will appear here."}
        </p>
        {!filtering && tunnelUrl ? (
          <code className="mt-4 rounded-lg border border-border bg-terminal px-3 py-2 font-mono text-xs text-terminal-foreground">
            curl {tunnelUrl}
          </code>
        ) : null}
      </div>
    )
  }

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="sticky top-0 z-10 flex items-center gap-3 border-b border-border bg-card/95 px-4 py-2.5 backdrop-blur sm:gap-4 sm:px-6">
        <span className={COL.chevron} aria-hidden />
        <span className={`${COL.time} text-[11px] font-medium uppercase tracking-wide text-muted-foreground`}>Time</span>
        <span className={`${COL.method} text-[11px] font-medium uppercase tracking-wide text-muted-foreground`}>Method</span>
        <span className={`${COL.path} text-[11px] font-medium uppercase tracking-wide text-muted-foreground`}>Path</span>
        <span className={`${COL.status} text-[11px] font-medium uppercase tracking-wide text-muted-foreground`}>Status</span>
        <span className={`${COL.duration} text-[11px] font-medium uppercase tracking-wide text-muted-foreground`}>Duration</span>
        <span className={COL.action} aria-hidden />
      </div>

      <div className="divide-y divide-border/60">
        {requests.map((request) => (
          <RequestRow key={request.id} request={request} onReplay={onReplay} tunnelUrl={tunnelUrl} />
        ))}
      </div>
    </div>
  )
}
