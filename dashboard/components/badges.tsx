"use client"

import { HttpMethod } from "@/lib/types"

const METHOD_COLOR: Record<string, string> = {
  GET: "var(--method-get)",
  POST: "var(--method-post)",
  PUT: "var(--method-put)",
  PATCH: "var(--method-patch)",
  DELETE: "var(--method-delete)",
}

/** Tinted pill: full-strength text on a 12% wash of the same hue. */
function tint(color: string) {
  return {
    color,
    backgroundColor: `color-mix(in srgb, ${color} 12%, transparent)`,
    boxShadow: `inset 0 0 0 1px color-mix(in srgb, ${color} 22%, transparent)`,
  }
}

export function MethodBadge({ method }: { method: HttpMethod | string }) {
  const color = METHOD_COLOR[method] ?? "var(--muted-foreground)"
  return (
    <span
      style={tint(color)}
      className="inline-flex h-6 w-full items-center justify-center rounded-md px-1.5 font-mono text-[11px] font-semibold tracking-wide"
    >
      {method}
    </span>
  )
}

export function statusColor(statusCode: number) {
  if (statusCode >= 500) return "var(--status-5xx)"
  if (statusCode >= 400) return "var(--status-4xx)"
  if (statusCode >= 300) return "var(--status-3xx)"
  if (statusCode >= 200) return "var(--status-2xx)"
  return "var(--muted-foreground)"
}

/**
 * The dot repeats the status class as shape as well as hue, so the 2xx/5xx
 * distinction is not carried by colour alone.
 */
export function StatusBadge({ statusCode }: { statusCode: number }) {
  const color = statusColor(statusCode)
  return (
    <span
      style={tint(color)}
      className="inline-flex h-6 items-center gap-1.5 rounded-full px-2 font-mono text-[11px] font-semibold"
    >
      <span className="size-1.5 shrink-0 rounded-full" style={{ backgroundColor: color }} aria-hidden />
      {statusCode || "--"}
    </span>
  )
}
