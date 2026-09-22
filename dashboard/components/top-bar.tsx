"use client"

import { useEffect, useRef, useState } from "react"
import { useTheme } from "next-themes"
import { Moon, Search, Sun, Trash2, X } from "lucide-react"
import { GoPortLogo } from "./goport-logo"

interface TopBarProps {
  requestCount: number
  filteredCount: number
  searchQuery: string
  onSearchChange: (query: string) => void
  onClearRequests: () => void
}

export function TopBar({ requestCount, filteredCount, searchQuery, onSearchChange, onClearRequests }: TopBarProps) {
  const { resolvedTheme, setTheme } = useTheme()
  const [mounted, setMounted] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => setMounted(true), [])

  // "/" focuses the filter, Escape clears it -- ignored while typing elsewhere.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = document.activeElement
      const typing = el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement
      if (e.key === "/" && !typing && !e.metaKey && !e.ctrlKey && !e.altKey) {
        e.preventDefault()
        inputRef.current?.focus()
      } else if (e.key === "Escape" && el === inputRef.current) {
        onSearchChange("")
        inputRef.current?.blur()
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [onSearchChange])

  const isDark = resolvedTheme === "dark"
  const filtering = searchQuery.trim().length > 0

  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b border-border bg-card/40 px-4 sm:px-5">
      <GoPortLogo className="h-5 w-auto shrink-0 text-foreground lg:hidden" />

      <h1 className="flex shrink-0 items-baseline gap-2">
        <span className="text-sm font-semibold text-foreground">Requests</span>
        <span className="font-mono text-xs tabular-nums text-muted-foreground" aria-live="polite">
          {filtering ? `${filteredCount}/${requestCount}` : requestCount}
        </span>
      </h1>

      <div className="relative mx-auto min-w-0 max-w-md flex-1">
        <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" aria-hidden />
        <input
          ref={inputRef}
          type="text"
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder="Filter by path, method, or status"
          aria-label="Filter requests by path, method, or status"
          className="h-9 w-full rounded-lg border border-border bg-background pl-8 pr-9 text-xs text-foreground placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/35"
        />
        {filtering ? (
          <button
            type="button"
            onClick={() => onSearchChange("")}
            aria-label="Clear filter"
            className="absolute right-1.5 top-1/2 inline-flex size-6 -translate-y-1/2 items-center justify-center rounded text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <X className="size-3" aria-hidden />
          </button>
        ) : (
          <kbd className="pointer-events-none absolute right-2 top-1/2 hidden -translate-y-1/2 rounded border border-border px-1 font-mono text-[10px] text-muted-foreground sm:block">
            /
          </kbd>
        )}
      </div>

      <div className="flex shrink-0 items-center gap-2">
        <button
          type="button"
          onClick={onClearRequests}
          disabled={requestCount === 0}
          className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-border px-2.5 text-xs font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-muted-foreground"
        >
          <Trash2 className="size-3.5" aria-hidden />
          <span className="max-sm:sr-only">Clear</span>
        </button>

        <button
          type="button"
          onClick={() => setTheme(isDark ? "light" : "dark")}
          className="inline-flex size-9 items-center justify-center rounded-lg border border-border text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={mounted ? `Switch to ${isDark ? "light" : "dark"} theme` : "Toggle theme"}
          title={mounted ? `Switch to ${isDark ? "light" : "dark"} theme` : "Toggle theme"}
        >
          {mounted ? (isDark ? <Sun className="size-4" aria-hidden /> : <Moon className="size-4" aria-hidden />) : <span className="size-4" />}
        </button>
      </div>
    </header>
  )
}
