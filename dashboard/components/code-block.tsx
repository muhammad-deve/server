"use client"

import { CopyButton } from "./copy-button"

interface CodeBlockProps {
  title: string
  content: string
}

export function CodeBlock({ title, content }: CodeBlockProps) {
  if (!content) {
    return (
      <div className="rounded-md bg-[var(--goport-bg)] border border-[var(--goport-border)] p-4">
        <div className="text-xs text-[var(--goport-text-muted)] italic">No {title.toLowerCase()}</div>
      </div>
    )
  }

  // Syntax highlighting for JSON
  const highlightJSON = (json: string) => {
    return json
      .replace(/"([^"]+)":/g, '<span class="text-[var(--goport-text-secondary)]">"$1"</span>:')
      .replace(/: "([^"]+)"/g, ': <span class="text-[var(--goport-success)]">"$1"</span>')
      .replace(/: (\d+)/g, ': <span class="text-[var(--goport-cyan)]">$1</span>')
      .replace(/: (true|false)/g, ': <span class="text-[var(--goport-warning)]">$1</span>')
      .replace(/: (null)/g, ': <span class="text-[var(--goport-text-muted)]">$1</span>')
  }

  const isJSON = content.trim().startsWith('{') || content.trim().startsWith('[')
  
  return (
    <div className="rounded-md bg-[var(--goport-bg)] border border-[var(--goport-border)] overflow-hidden">
      <div className="flex items-center justify-between px-3 py-2 bg-[var(--goport-bg-secondary)] border-b border-[var(--goport-border)]">
        <span className="text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">{title}</span>
        <CopyButton text={content} />
      </div>
      <div className="p-3 overflow-x-auto">
        <pre className="text-xs leading-relaxed font-mono">
          {isJSON ? (
            <code dangerouslySetInnerHTML={{ __html: highlightJSON(content) }} />
          ) : (
            <code className="text-[var(--goport-text-secondary)]">{content}</code>
          )}
        </pre>
      </div>
    </div>
  )
}

interface HeadersBlockProps {
  headers: Record<string, string>
}

export function HeadersBlock({ headers }: HeadersBlockProps) {
  const headersText = Object.entries(headers)
    .map(([key, value]) => `${key}: ${value}`)
    .join('\n')

  return (
    <div className="rounded-md bg-[var(--goport-bg)] border border-[var(--goport-border)] overflow-hidden">
      <div className="flex items-center justify-between px-3 py-2 bg-[var(--goport-bg-secondary)] border-b border-[var(--goport-border)]">
        <span className="text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">Headers</span>
        <CopyButton text={headersText} />
      </div>
      <div className="p-3 overflow-x-auto">
        <div className="space-y-0.5">
          {Object.entries(headers).map(([key, value]) => (
            <div key={key} className="text-xs font-mono">
              <span className="text-[var(--goport-text-secondary)]">{key}</span>
              <span className="text-[var(--goport-text-muted)]">: </span>
              <span className="text-[var(--goport-text-muted)]">{value}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

interface CurlBlockProps {
  curl: string
}

export function CurlBlock({ curl }: CurlBlockProps) {
  // Highlight curl command
  const highlightCurl = (cmd: string) => {
    return cmd
      .replace(/^(curl)/m, '<span class="text-[var(--goport-cyan)]">$1</span>')
      .replace(/(-X\s+)(\w+)/g, '$1<span class="text-[var(--goport-violet)]">$2</span>')
      .replace(/(-H\s+)"([^"]+)"/g, '$1<span class="text-[var(--goport-text-secondary)]">"$2"</span>')
      .replace(/(-d\s+)'([^']+)'/g, '$1<span class="text-[var(--goport-success)]">\'$2\'</span>')
      .replace(/(https?:\/\/[^\s\\]+)/g, '<span class="text-[var(--goport-text-secondary)]">$1</span>')
  }

  return (
    <div className="rounded-md bg-[var(--goport-bg)] border border-[var(--goport-border)] overflow-hidden">
      <div className="flex items-center justify-between px-3 py-2 bg-[var(--goport-bg-secondary)] border-b border-[var(--goport-border)]">
        <span className="text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">Command</span>
        <CopyButton text={curl} />
      </div>
      <div className="p-3 overflow-x-auto">
        <pre className="text-xs leading-relaxed font-mono">
          <code dangerouslySetInnerHTML={{ __html: highlightCurl(curl) }} />
        </pre>
      </div>
    </div>
  )
}
