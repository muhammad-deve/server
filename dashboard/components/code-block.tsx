"use client"

import { useMemo, useState } from "react"
import { WrapText } from "lucide-react"
import { CopyButton } from "./copy-button"

/**
 * Request and response bodies arrive from the public internet, and anyone who
 * knows a tunnel URL can choose their contents. Escape before building any
 * highlighted markup: without this, a body of `<img src=x onerror=...>` runs
 * inside the inspector, where it can replay or clear the user's requests.
 *
 * Escaping `&`, `<` and `>` is sufficient because the result is only ever
 * inserted in text-node position, never into an attribute value.
 */
function escapeHtml(s: string) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

const JSON_TOKEN =
  /("(?:\\u[a-fA-F0-9]{4}|\\[^u]|[^\\"])*"(?:\s*:)?|\b(?:true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g

/** Tokenises the ALREADY-escaped string, so no raw user input reaches the DOM. */
function highlightJSON(escaped: string) {
  return escaped.replace(JSON_TOKEN, (match) => {
    let cls = "json-number"
    if (match.startsWith('"')) cls = match.trimEnd().endsWith(":") ? "json-key" : "json-string"
    else if (match === "true" || match === "false") cls = "json-bool"
    else if (match === "null") cls = "json-null"
    return `<span class="${cls}">${match}</span>`
  })
}

function tryPrettyJSON(content: string): string | null {
  const trimmed = content.trim()
  if (!(trimmed.startsWith("{") || trimmed.startsWith("["))) return null
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return null
  }
}

function Panel({
  title,
  meta,
  actions,
  children,
}: {
  title: string
  meta?: React.ReactNode
  actions?: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <section className="overflow-hidden rounded-xl border border-border bg-card">
      <header className="flex min-h-10 items-center justify-between gap-3 border-b border-border px-3.5 py-1.5">
        <div className="flex min-w-0 items-center gap-2">
          <span className="text-xs font-semibold text-foreground">{title}</span>
          {meta ? <span className="truncate text-xs text-muted-foreground">{meta}</span> : null}
        </div>
        <div className="flex items-center gap-1">{actions}</div>
      </header>
      {children}
    </section>
  )
}

export function CodeBlock({ title, content }: { title: string; content: string }) {
  const [wrap, setWrap] = useState(true)
  const pretty = useMemo(() => (content ? tryPrettyJSON(content) : null), [content])
  const size = useMemo(() => new Blob([content || ""]).size, [content])
  const highlighted = useMemo(() => (pretty ? highlightJSON(escapeHtml(pretty)) : null), [pretty])

  if (!content) {
    return (
      <Panel title={title}>
        <p className="px-3.5 py-4 text-xs text-muted-foreground">No {title.toLowerCase()} for this request.</p>
      </Panel>
    )
  }

  return (
    <Panel
      title={title}
      meta={`${pretty ? "JSON · " : ""}${formatBytes(size)}`}
      actions={
        <>
          <button
            type="button"
            onClick={() => setWrap((w) => !w)}
            aria-pressed={wrap}
            title={wrap ? "Turn off line wrapping" : "Wrap long lines"}
            aria-label={wrap ? "Turn off line wrapping" : "Wrap long lines"}
            className={`inline-flex size-7 items-center justify-center rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
              wrap ? "bg-secondary text-foreground" : "text-muted-foreground hover:bg-secondary hover:text-foreground"
            }`}
          >
            <WrapText className="size-3.5" aria-hidden />
          </button>
          <CopyButton text={pretty ?? content} label={title.toLowerCase()} />
        </>
      }
    >
      {/* Large bodies scroll inside the panel rather than being clipped off. */}
      <div className="max-h-[min(60vh,32rem)] overflow-auto bg-terminal">
        <pre
          className={`p-3.5 font-mono text-xs leading-relaxed text-terminal-foreground ${
            wrap ? "whitespace-pre-wrap break-all" : "w-max min-w-full whitespace-pre"
          }`}
        >
          {highlighted ? <code dangerouslySetInnerHTML={{ __html: highlighted }} /> : <code>{content}</code>}
        </pre>
      </div>
    </Panel>
  )
}

export function HeadersBlock({ headers }: { headers: Record<string, string> }) {
  const entries = Object.entries(headers).sort(([a], [b]) => a.localeCompare(b))
  const headersText = entries.map(([k, v]) => `${k}: ${v}`).join("\n")

  return (
    <Panel
      title="Headers"
      meta={entries.length ? String(entries.length) : undefined}
      actions={entries.length ? <CopyButton text={headersText} label="headers" /> : null}
    >
      {entries.length === 0 ? (
        <p className="px-3.5 py-4 text-xs text-muted-foreground">No headers.</p>
      ) : (
        <div className="max-h-[min(40vh,20rem)] overflow-auto">
          <dl className="divide-y divide-border/60">
            {entries.map(([key, value]) => (
              <div key={key} className="grid grid-cols-[minmax(0,10rem)_minmax(0,1fr)] gap-4 px-3.5 py-1.5 font-mono text-xs">
                <dt className="truncate font-medium text-foreground" title={key}>{key}</dt>
                <dd className="break-all text-muted-foreground">{value}</dd>
              </div>
            ))}
          </dl>
        </div>
      )}
    </Panel>
  )
}

/** Same rule as the JSON path: escape first, then decorate. */
function highlightCurl(cmd: string) {
  return escapeHtml(cmd)
    .replace(/^curl/, '<span class="curl-cmd">curl</span>')
    .replace(/(^|\s)(-X|-H|-d)(\s)/g, '$1<span class="curl-flag">$2</span>$3')
}

export function CurlBlock({ curl }: { curl: string }) {
  const highlighted = useMemo(() => highlightCurl(curl), [curl])
  return (
    <Panel title="cURL" meta="Re-run this request from your terminal" actions={<CopyButton text={curl} label="command" />}>
      <div className="max-h-[min(60vh,32rem)] overflow-auto bg-terminal">
        <pre className="whitespace-pre-wrap break-all p-3.5 font-mono text-xs leading-relaxed text-terminal-foreground">
          <code dangerouslySetInnerHTML={{ __html: highlighted }} />
        </pre>
      </div>
    </Panel>
  )
}
