"use client"

import { useState, useEffect } from "react"
import { useTheme } from "next-themes"
import { Search, Trash2, ExternalLink, Sun, Moon } from "lucide-react"
import { CopyButton } from "./copy-button"

interface TopBarProps {
  tunnelUrl: string
  searchQuery: string
  onSearchChange: (query: string) => void
  onClearRequests: () => void
  requestCount: number
}

export function TopBar({ 
  tunnelUrl, 
  searchQuery, 
  onSearchChange, 
  onClearRequests,
  requestCount 
}: TopBarProps) {
  const { theme, setTheme } = useTheme()
  const [mounted, setMounted] = useState(false)

  // Wait for client-side hydration before rendering theme-dependent UI
  useEffect(() => {
    setMounted(true)
  }, [])

  return (
    <header className="h-14 bg-[var(--goport-bg-secondary)] border-b border-[var(--goport-border)] flex items-center justify-between px-5">
      {/* Tunnel URL Display */}
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-3 px-3 py-1.5 rounded-md bg-[var(--goport-bg-tertiary)] border border-[var(--goport-border)]">
          <div className="w-1.5 h-1.5 rounded-full bg-[var(--goport-success)]"></div>
          <code className="text-sm text-[var(--goport-text-secondary)] font-mono">
            {tunnelUrl}
          </code>
          <CopyButton text={tunnelUrl} />
          <a
            href={tunnelUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="p-1 rounded hover:bg-[var(--goport-border)] transition-colors"
          >
            <ExternalLink className="w-3.5 h-3.5 text-[var(--goport-text-muted)] hover:text-[var(--goport-text-secondary)]" />
          </a>
        </div>
        <span className="text-xs text-[var(--goport-text-muted)]">
          {requestCount} request{requestCount !== 1 ? 's' : ''}
        </span>
      </div>

      {/* Search and Actions */}
      <div className="flex items-center gap-2">
        {/* Search Bar */}
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-[var(--goport-text-muted)]" />
          <input
            type="text"
            placeholder="Filter by path, status, or method..."
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            className="w-64 pl-9 pr-3 py-1.5 rounded-md bg-[var(--goport-bg-tertiary)] border border-[var(--goport-border)] text-sm text-[var(--goport-text-secondary)] placeholder:text-[var(--goport-text-muted)] focus:outline-none focus:border-[var(--goport-border-subtle)] transition-colors font-mono"
          />
        </div>

        {/* Clear Button */}
        <button
          onClick={onClearRequests}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-[var(--goport-bg-tertiary)] border border-[var(--goport-border)] text-xs text-[var(--goport-text-muted)] hover:text-[var(--goport-text-secondary)] hover:border-[var(--goport-border-subtle)] transition-colors"
        >
          <Trash2 className="w-3.5 h-3.5" />
          <span>Clear</span>
        </button>

        {/* Theme Toggle */}
        <button
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
          className="p-2 rounded-md bg-[var(--goport-bg-tertiary)] border border-[var(--goport-border)] text-[var(--goport-text-muted)] hover:text-[var(--goport-text-secondary)] hover:border-[var(--goport-border-subtle)] transition-colors"
          title={mounted ? `Switch to ${theme === "dark" ? "light" : "dark"} mode` : "Toggle theme"}
          aria-label="Toggle theme"
        >
          {mounted ? (
            theme === "dark" ? (
              <Sun className="w-4 h-4" />
            ) : (
              <Moon className="w-4 h-4" />
            )
          ) : (
            <div className="w-4 h-4" />
          )}
        </button>
      </div>
    </header>
  )
}
