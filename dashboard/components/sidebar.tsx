"use client"

import { Globe, Activity, MapPin, Gauge, BarChart3 } from "lucide-react"
import { CopyButton } from "./copy-button"
import { TunnelData } from "@/lib/types"

interface SidebarProps {
  tunnelData: TunnelData
}

export function Sidebar({ tunnelData }: SidebarProps) {
  const dashboardHost = typeof window !== "undefined" ? window.location.host : "localhost:4040"
  const displayUrl = tunnelData.url ? tunnelData.url.replace(/^https?:\/\//, "") : "—"

  return (
    <aside className="w-64 min-h-screen bg-[var(--goport-bg-secondary)] border-r border-[var(--goport-border)] flex flex-col">
      {/* Logo */}
      <div className="p-5 border-b border-[var(--goport-border)]">
        <div className="flex items-center gap-3">
          <svg
            width="32"
            height="32"
            viewBox="0 0 64 64"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
            className="rounded"
            aria-label="GoPort"
          >
            <rect width="64" height="64" rx="14" fill="#050505" />
            <path d="M13 32H51" stroke="#25F59A" strokeWidth="5" strokeLinecap="round" />
            <path d="M34 20L48 32L34 44" stroke="#25F59A" strokeWidth="5" strokeLinecap="round" strokeLinejoin="round" />
            <path d="M16 20H26C32.6 20 38 25.4 38 32C38 38.6 32.6 44 26 44H16" stroke="#FFFFFF" strokeWidth="5" strokeLinecap="round" />
            <circle cx="16" cy="32" r="4" fill="#25F59A" />
          </svg>
          <div>
            <h1 className="text-base font-semibold text-[var(--goport-text)]">GoPort</h1>
            <p className="text-xs text-[var(--goport-text-muted)]">{dashboardHost}</p>
          </div>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="flex-1 p-3 space-y-2 overflow-y-auto">
        {/* Tunnel URL */}
        <div className="p-3 rounded-lg bg-[var(--goport-bg-tertiary)] border border-[var(--goport-border)]">
          <div className="flex items-center gap-2 mb-2">
            <Globe className="w-3.5 h-3.5 text-[var(--goport-text-muted)]" />
            <span className="text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">Tunnel URL</span>
          </div>
          <div className="flex items-center gap-2">
            <code className="text-xs text-[var(--goport-text-secondary)] truncate flex-1 font-mono">
              {displayUrl}
            </code>
            {tunnelData.url ? <CopyButton text={tunnelData.url} /> : null}
          </div>
        </div>

        {/* Status */}
        <div className="p-3 rounded-lg bg-[var(--goport-bg-tertiary)] border border-[var(--goport-border)]">
          <div className="flex items-center gap-2 mb-2">
            <Activity className="w-3.5 h-3.5 text-[var(--goport-text-muted)]" />
            <span className="text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">Status</span>
          </div>
          <div className="flex items-center gap-2">
            <div
              className={`w-2 h-2 rounded-full ${
                tunnelData.status === "online"
                  ? "bg-[var(--goport-success)] animate-pulse-dot"
                  : tunnelData.status === "connecting"
                  ? "bg-[var(--goport-warning)] animate-pulse-dot"
                  : "bg-[var(--goport-error)]"
              }`}
            ></div>
            <span className="text-sm text-[var(--goport-text-secondary)] capitalize">{tunnelData.status}</span>
          </div>
        </div>

        {/* Region */}
        <div className="p-3 rounded-lg bg-[var(--goport-bg-tertiary)] border border-[var(--goport-border)]">
          <div className="flex items-center gap-2 mb-2">
            <MapPin className="w-3.5 h-3.5 text-[var(--goport-text-muted)]" />
            <span className="text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">Region</span>
          </div>
          <span className="text-sm text-[var(--goport-text-secondary)]">{tunnelData.region || "—"}</span>
        </div>

        {/* Latency */}
        <div className="p-3 rounded-lg bg-[var(--goport-bg-tertiary)] border border-[var(--goport-border)]">
          <div className="flex items-center gap-2 mb-2">
            <Gauge className="w-3.5 h-3.5 text-[var(--goport-text-muted)]" />
            <span className="text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">Latency</span>
          </div>
          <div className="flex items-baseline gap-1">
            <span className="text-xl font-semibold text-[var(--goport-text)]">{tunnelData.latency}</span>
            <span className="text-xs text-[var(--goport-text-muted)]">ms</span>
          </div>
        </div>

        {/* Total Requests */}
        <div className="p-3 rounded-lg bg-[var(--goport-bg-tertiary)] border border-[var(--goport-border)]">
          <div className="flex items-center gap-2 mb-2">
            <BarChart3 className="w-3.5 h-3.5 text-[var(--goport-text-muted)]" />
            <span className="text-xs text-[var(--goport-text-muted)] uppercase tracking-wide">Total Requests</span>
          </div>
          <span className="text-xl font-semibold text-[var(--goport-text)]">{tunnelData.requestsToday.toLocaleString()}</span>
        </div>
      </div>

      {/* Footer */}
      <div className="p-3 border-t border-[var(--goport-border)]">
        <div className="text-xs text-[var(--goport-text-muted)] text-center">
          GoPort
        </div>
      </div>
    </aside>
  )
}
