"use client"

import { Activity, ArrowUpRight, Gauge, HardDrive, MapPin } from "lucide-react"
import { CopyButton } from "./copy-button"
import { GoPortLogo } from "./goport-logo"
import { PlanInfo, TunnelData } from "@/lib/types"

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

const STATUS = {
  online: { label: "Online", color: "var(--primary)", pulse: true },
  connecting: { label: "Connecting", color: "var(--status-4xx)", pulse: true },
  offline: { label: "Offline", color: "var(--status-5xx)", pulse: false },
} as const

/** One metric tile. Value is mono so the 2x2 grid stays on a common baseline. */
function Tile({
  icon: Icon,
  label,
  value,
  suffix,
}: {
  icon: typeof Gauge
  label: string
  value: string
  suffix?: string
}) {
  return (
    <div className="min-w-0 rounded-lg border border-border bg-card px-2.5 py-2">
      <div className="flex items-center gap-1.5">
        <Icon className="size-3 shrink-0 text-primary" aria-hidden />
        <span className="truncate text-[10px] font-medium uppercase tracking-[0.07em] text-muted-foreground">
          {label}
        </span>
      </div>
      <p className="mt-1 flex items-baseline gap-1">
        <span className="truncate font-mono text-sm font-semibold tabular-nums text-foreground" title={value}>
          {value}
        </span>
        {suffix ? <span className="shrink-0 text-[11px] text-muted-foreground">{suffix}</span> : null}
      </p>
    </div>
  )
}

/**
 * Monthly allowance. Free includes 5 GB and Pro 70 GB, and the used figure is
 * the same month-to-date number the server measures the limit against -- so
 * what this shows is what will actually stop a tunnel opening.
 *
 * Renders nothing until the API has answered, rather than guessing a quota.
 */
function PlanCard({ plan }: { plan?: PlanInfo }) {
  if (!plan?.known) return null

  const used = Math.max(plan.monthBytes, 0)
  const limit = Math.max(plan.monthlyBytes, 1)
  const percent = Math.min((used / limit) * 100, 100)
  const nearLimit = percent >= 90

  return (
    <section className="rounded-lg border border-border bg-card p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-[10px] font-medium uppercase tracking-[0.07em] text-muted-foreground">Plan</span>
        <span
          className={`rounded-md px-1.5 py-0.5 text-[10px] font-semibold ${
            plan.isPro ? "bg-primary/12 text-primary" : "bg-secondary text-muted-foreground"
          }`}
        >
          {plan.isPro ? "Pro" : "Free"}
        </span>
      </div>

      <p className="mt-2 font-mono text-sm font-semibold tabular-nums text-foreground">
        {formatBytes(used)}
        <span className="font-sans text-xs font-normal text-muted-foreground"> of {formatBytes(limit)}</span>
      </p>

      <div
        className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-secondary"
        role="progressbar"
        aria-valuenow={Math.round(percent)}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label="Monthly traffic used"
      >
        <div
          className={`h-full rounded-full ${nearLimit ? "bg-destructive" : "bg-primary"}`}
          style={{ width: `${Math.max(percent, percent > 0 ? 2 : 0)}%` }}
        />
      </div>

      {/* Percentage in text too, so the bar is not the only signal. */}
      <p className="mt-1.5 text-[11px] text-muted-foreground">
        {percent < 1 && used > 0 ? "under 1%" : `${Math.round(percent)}%`} used this month
      </p>
    </section>
  )
}

export function Sidebar({ tunnelData }: SidebarProps) {
  const host = tunnelData.url ? tunnelData.url.replace(/^https?:\/\//, "") : ""
  const status = STATUS[tunnelData.status] ?? STATUS.offline
  const dashboardHost = typeof window !== "undefined" ? window.location.host : "127.0.0.1:4040"

  return (
    <aside className="flex w-72 shrink-0 flex-col border-r border-border bg-card/40 max-lg:hidden">
      {/* h-14 matches the main top bar so the two bottom borders form one line. */}
      <div className="flex h-14 shrink-0 items-center gap-2 border-b border-border px-4">
        <GoPortLogo className="h-[21px] w-auto text-foreground" />
        <span className="rounded-md border border-border px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
          Inspector
        </span>
      </div>

      <div className="flex-1 space-y-2.5 overflow-y-auto p-3">

      {/* Tunnel card: status, the public URL, and its two direct actions. */}
      <section className="rounded-lg border border-border bg-card p-3">
        <div className="flex items-center gap-1.5">
          <span className="inline-flex items-center gap-1.5">
            <span
              className={`size-2 shrink-0 rounded-full ${status.pulse ? "animate-pulse-dot" : ""}`}
              style={{ backgroundColor: status.color }}
              aria-hidden
            />
            <span className="text-xs font-semibold text-foreground">{status.label}</span>
          </span>
        </div>

        <p className="mt-2.5 text-[11px] text-muted-foreground">Public URL</p>
        {host ? (
          <>
            <p className="mt-1 truncate font-mono text-sm font-semibold text-foreground" title={tunnelData.url}>
              {host}
            </p>
            <div className="mt-2.5 grid grid-cols-2 gap-1.5">
              <CopyButton
                text={tunnelData.url}
                label="URL"
                className="h-8 w-full justify-center border border-border bg-background hover:bg-secondary"
              />
              <a
                href={tunnelData.url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex h-8 items-center justify-center gap-1 rounded-lg bg-primary px-2 text-xs font-semibold text-primary-foreground transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                Open
                <ArrowUpRight className="size-3.5" aria-hidden />
              </a>
            </div>
          </>
        ) : (
          <p className="mt-1 font-mono text-sm text-muted-foreground">connecting…</p>
        )}

        <p className="mt-3 truncate font-mono text-[11px] text-muted-foreground" title={dashboardHost}>
          &rarr; {dashboardHost}
        </p>
      </section>

      <div className="grid grid-cols-2 gap-2">
        <Tile icon={Activity} label="Requests" value={tunnelData.requestsToday.toLocaleString()} />
        <Tile icon={HardDrive} label="Transfer" value={formatBytes(tunnelData.totalBytes)} />
        <Tile icon={Gauge} label="Latency" value={String(tunnelData.latency)} suffix="ms" />
        <Tile icon={MapPin} label="Region" value={tunnelData.region || "--"} />
      </div>

      </div>

      <div className="flex shrink-0 items-center justify-between gap-2 border-t border-border px-4 py-3 text-xs text-muted-foreground">
        <span className="font-mono">GoPort{tunnelData.version ? ` v${tunnelData.version}` : ""}</span>
        <a
          href="https://goport.uz/dashboard"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          Account
          <ArrowUpRight className="size-3.5" aria-hidden />
        </a>
      </div>
    </aside>
  )
}
