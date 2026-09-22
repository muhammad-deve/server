"use client"

import { ExternalLink, Gauge, Globe, MapPin, Activity } from "lucide-react"
import { CopyButton } from "./copy-button"
import { GoPortLogo } from "./goport-logo"
import { TunnelData } from "@/lib/types"

interface SidebarProps {
  tunnelData: TunnelData
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const value = bytes / Math.pow(1024, i)
  return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[i]}`
}

const STATUS_COPY: Record<TunnelData["status"], { label: string; color: string; pulse: boolean }> = {
  online: { label: "Online", color: "var(--primary)", pulse: true },
  connecting: { label: "Connecting", color: "var(--status-4xx)", pulse: true },
  offline: { label: "Offline", color: "var(--status-5xx)", pulse: false },
}

function Stat({ icon: Icon, label, children }: { icon: typeof Gauge; label: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-border bg-card p-3">
      <div className="flex items-center gap-2">
        <Icon className="size-3.5 text-muted-foreground" aria-hidden />
        <span className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{label}</span>
      </div>
      <div className="mt-2">{children}</div>
    </div>
  )
}

export function Sidebar({ tunnelData }: SidebarProps) {
  const displayUrl = tunnelData.url ? tunnelData.url.replace(/^https?:\/\//, "") : "--"
  const status = STATUS_COPY[tunnelData.status] ?? STATUS_COPY.offline

  return (
    <aside className="flex w-60 shrink-0 flex-col border-r border-border bg-card/60 max-lg:hidden">
      <div className="flex h-16 items-center border-b border-border px-5">
        <GoPortLogo className="h-6 w-auto text-foreground" />
      </div>

      <div className="flex-1 space-y-2.5 overflow-y-auto p-3">
        <Stat icon={Globe} label="Tunnel URL">
          <div className="flex items-center gap-1.5">
            <span
              className={`size-2 shrink-0 rounded-full ${status.pulse ? "animate-pulse-dot" : ""}`}
              style={{ backgroundColor: status.color }}
              aria-hidden
            />
            {/* Status is stated in text too, not carried by the dot's colour alone. */}
            <span className="sr-only">{status.label}.</span>
            <code className="min-w-0 flex-1 truncate font-mono text-xs text-foreground" title={displayUrl}>
              {displayUrl}
            </code>
          </div>
          {tunnelData.url ? (
            <div className="mt-2 flex items-center gap-1">
              <CopyButton text={tunnelData.url} label="URL" />
              <a
                href={tunnelData.url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex h-7 items-center gap-1.5 rounded-lg px-2 text-xs font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <ExternalLink className="size-3.5" aria-hidden />
                Open
              </a>
            </div>
          ) : null}
        </Stat>

        <div className="grid grid-cols-2 gap-2.5">
          <Stat icon={MapPin} label="Region">
            <span className="text-sm font-medium text-foreground">{tunnelData.region || "--"}</span>
          </Stat>
          <Stat icon={Gauge} label="Latency">
            <span className="font-mono text-lg font-semibold tabular-nums text-foreground">{tunnelData.latency}</span>
            <span className="ml-1 text-xs text-muted-foreground">ms</span>
          </Stat>
        </div>

        <Stat icon={Activity} label="Requests">
          <div className="flex items-baseline gap-2">
            <span className="font-mono text-lg font-semibold tabular-nums text-foreground">
              {tunnelData.requestsToday.toLocaleString()}
            </span>
            <span className="text-xs text-muted-foreground">{formatBytes(tunnelData.totalBytes)}</span>
          </div>
        </Stat>
      </div>

      <div className="flex items-center justify-between border-t border-border px-4 py-3 text-[11px] text-muted-foreground">
        <span className="font-mono">GoPort{tunnelData.version ? ` v${tunnelData.version}` : ""}</span>
        <a
          href="https://goport.uz/dashboard"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 transition-colors hover:text-foreground"
        >
          Account
          <ExternalLink className="size-3" aria-hidden />
        </a>
      </div>
    </aside>
  )
}
