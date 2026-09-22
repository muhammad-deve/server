"use client"

import { useEffect, useRef, useState } from "react"
import { Check, Copy } from "lucide-react"

interface CopyButtonProps {
  text: string
  /** When set, the button shows a text label and names itself "Copy <label>". */
  label?: string
  className?: string
}

async function writeClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    // The async clipboard API needs a secure context. The inspector is served
    // over plain http on 127.0.0.1, which most browsers treat as secure, but
    // not all -- so fall back to the legacy path rather than failing silently.
    try {
      const textarea = document.createElement("textarea")
      textarea.value = text
      textarea.style.position = "fixed"
      textarea.style.opacity = "0"
      document.body.appendChild(textarea)
      textarea.select()
      const ok = document.execCommand("copy")
      document.body.removeChild(textarea)
      return ok
    } catch {
      return false
    }
  }
}

export function CopyButton({ text, label, className = "" }: CopyButtonProps) {
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => () => { if (timer.current) clearTimeout(timer.current) }, [])

  const handleCopy = async (e: React.MouseEvent) => {
    e.stopPropagation()
    if (!(await writeClipboard(text))) return
    setCopied(true)
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => setCopied(false), 1600)
  }

  return (
    <button
      type="button"
      onClick={handleCopy}
      title={copied ? "Copied" : "Copy to clipboard"}
      aria-label={copied ? "Copied" : label ? `Copy ${label}` : "Copy to clipboard"}
      className={`inline-flex h-7 shrink-0 items-center gap-1.5 rounded-lg px-2 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
        copied ? "bg-primary/12 text-primary" : "text-muted-foreground hover:bg-secondary hover:text-foreground"
      } ${className}`}
    >
      {copied ? <Check className="size-3.5" aria-hidden /> : <Copy className="size-3.5" aria-hidden />}
      {label ? <span>{copied ? "Copied" : "Copy"}</span> : null}
    </button>
  )
}
