"use client"

import { Globe, MapPin, Gauge, BarChart3, ExternalLink } from "lucide-react"
import { CopyButton } from "./copy-button"
import { GoPortLogo } from "./goport-logo"
import { TunnelData } from "@/lib/types"

interface SidebarProps {
  tunnelData: TunnelData
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const value = bytes / Math.pow(1024, i)
  return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[i]}`
}

export function Sidebar({ tunnelData }: SidebarProps) {
  const dashboardHost = typeof window !== "undefined" ? window.location.host : "localhost:4040"
  const displayUrl = tunnelData.url ? tunnelData.url.replace(/^https?:\/\//, "") : "—"

  return (
    <aside className="w-64 min-h-screen bg-[var(--goport-bg-secondary)] border-r border-[var(--goport-border)] flex flex-col">
      {/* Logo */}
      <div className="h-14 px-5 flex items-center border-b border-[var(--goport-border)]">
        <div className="flex flex-col gap-0.5">
          <GoPortLogo className="h-6 w-28" />
          <p className="text-xs text-[var(--goport-text-muted)] leading-tight">{dashboardHost}</p>
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
            <div
              className={`w-2 h-2 rounded-full shrink-0 ${
                tunnelData.status === "online"
                  ? "bg-[var(--goport-success)] animate-pulse-dot"
                  : tunnelData.status === "connecting"
                  ? "bg-[var(--goport-warning)] animate-pulse-dot"
                  : "bg-[var(--goport-error)]"
              }`}
              title={tunnelData.status}
            ></div>
            <code className="text-xs text-[var(--goport-text-secondary)] truncate flex-1 font-mono">
              {displayUrl}
            </code>
            {tunnelData.url ? (
              <>
                <CopyButton text={tunnelData.url} />
                <a
                  href={tunnelData.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="p-1 rounded hover:bg-[var(--goport-border)] transition-colors"
                  aria-label="Open tunnel URL"
                >
                  <ExternalLink className="w-3.5 h-3.5 text-[var(--goport-text-muted)] hover:text-[var(--goport-text-secondary)]" />
                </a>
              </>
            ) : null}
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
          <div className="flex items-baseline gap-2">
            <span className="text-xl font-semibold text-[var(--goport-text)]">{tunnelData.requestsToday.toLocaleString()}</span>
            <span className="text-sm text-[var(--goport-text-muted)]">({formatBytes(tunnelData.totalBytes)})</span>
          </div>
        </div>
      </div>

      {/* Footer */}
      <div className="p-3 border-t border-[var(--goport-border)]">
        <div className="text-xs text-[var(--goport-text-muted)] text-center">
          GoPort{tunnelData.version ? ` v${tunnelData.version}` : ""}
        </div>
      </div>
    </aside>
  )
}
