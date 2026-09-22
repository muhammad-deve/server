"use client"

import { useEffect, useRef, useState } from "react"
import { useTheme } from "next-themes"
import { Moon, Search, Sun, Trash2 } from "lucide-react"
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

  // "/" focuses the filter, unless the user is already typing somewhere.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "/" || e.metaKey || e.ctrlKey || e.altKey) return
      const el = document.activeElement
      if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) return
      e.preventDefault()
      inputRef.current?.focus()
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [])

  const isDark = resolvedTheme === "dark"
  const filtering = searchQuery.trim().length > 0

  return (
    <header className="flex h-16 shrink-0 items-center gap-3 border-b border-border bg-card/60 px-4 sm:px-6">
      <GoPortLogo className="h-6 w-auto shrink-0 text-foreground lg:hidden" />

      <div className="relative min-w-0 flex-1 sm:max-w-md">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
        <input
          ref={inputRef}
          type="search"
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder="Filter by path, method, or status"
          aria-label="Filter requests by path, method, or status"
          className="h-10 w-full rounded-xl border border-border bg-background pl-9 pr-10 text-sm text-foreground placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/40"
        />
        <kbd className="pointer-events-none absolute right-3 top-1/2 hidden -translate-y-1/2 rounded border border-border px-1.5 font-mono text-[10px] text-muted-foreground sm:block">
          /
        </kbd>
      </div>

      <p className="ml-auto hidden text-xs tabular-nums text-muted-foreground md:block" aria-live="polite">
        {filtering ? `${filteredCount} of ${requestCount}` : `${requestCount} request${requestCount === 1 ? "" : "s"}`}
      </p>

      <button
        type="button"
        onClick={onClearRequests}
        disabled={requestCount === 0}
        className="inline-flex h-10 shrink-0 items-center gap-1.5 rounded-xl border border-border bg-background px-3 text-xs font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-45 disabled:hover:bg-background disabled:hover:text-muted-foreground"
      >
        <Trash2 className="size-3.5" aria-hidden />
        <span className="max-sm:sr-only">Clear</span>
      </button>

      <button
        type="button"
        onClick={() => setTheme(isDark ? "light" : "dark")}
        className="inline-flex size-10 shrink-0 items-center justify-center rounded-xl border border-border bg-background text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        aria-label={mounted ? `Switch to ${isDark ? "light" : "dark"} theme` : "Toggle theme"}
        title={mounted ? `Switch to ${isDark ? "light" : "dark"} theme` : "Toggle theme"}
      >
        {/* Render a placeholder until mounted so the icon cannot mismatch on hydration. */}
        {mounted ? (isDark ? <Sun className="size-4" aria-hidden /> : <Moon className="size-4" aria-hidden />) : <span className="size-4" />}
      </button>
    </header>
  )
}
