"use client"

import { useState, useMemo, useEffect, useCallback } from "react"
import { Sidebar } from "@/components/sidebar"
import { TopBar } from "@/components/top-bar"
import { RequestsTable } from "@/components/requests-table"
import { HttpRequest, TunnelData } from "@/lib/types"
import {
  fetchTunnel,
  fetchRequests,
  clearRequests as apiClearRequests,
  replayRequest as apiReplayRequest,
  subscribeRequests,
} from "@/lib/api"

const defaultTunnel: TunnelData = {
  url: "",
  status: "connecting",
  region: "",
  latency: 0,
  requestsToday: 0,
  totalBytes: 0,
  version: "",
}

export default function Dashboard() {
  const [tunnel, setTunnel] = useState<TunnelData>(defaultTunnel)
  const [requests, setRequests] = useState<HttpRequest[]>([])
  const [searchQuery, setSearchQuery] = useState("")

  // Initial load: tunnel info + recent requests, then keep tunnel info refreshed.
  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const [t, rs] = await Promise.all([fetchTunnel(), fetchRequests()])
        if (cancelled) return
        setTunnel(t)
        setRequests(rs)
      } catch {
        if (!cancelled) setTunnel((prev) => ({ ...prev, status: "offline" }))
      }
    }
    load()
    const id = setInterval(async () => {
      try {
        const t = await fetchTunnel()
        if (!cancelled) setTunnel(t)
      } catch {
        // ignore transient failures
      }
    }, 5000)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [])

  // Live stream new requests via SSE. This is the single source of truth for
  // the list: every captured request (including replays) arrives here exactly
  // once, and we dedupe by id so nothing shows up twice.
  useEffect(() => {
    const teardown = subscribeRequests((req) => {
      setRequests((prev) => {
        if (prev.some((r) => r.id === req.id)) return prev
        return [req, ...prev].slice(0, 500)
      })
    })
    return teardown
  }, [])

  const filteredRequests = useMemo(() => {
    if (!searchQuery.trim()) return requests
    const query = searchQuery.toLowerCase().trim()
    return requests.filter((request) => {
      if (request.path.toLowerCase().includes(query)) return true
      if (request.statusCode.toString().includes(query)) return true
      if (request.method.toLowerCase().includes(query)) return true
      return false
    })
  }, [requests, searchQuery])

  const handleClearRequests = useCallback(async () => {
    setRequests([])
    try {
      await apiClearRequests()
    } catch {
      // refetch on failure to stay consistent
      const rs = await fetchRequests().catch(() => [])
      setRequests(rs)
    }
  }, [])

  const handleReplay = useCallback(async (request: HttpRequest) => {
    try {
      // The replayed request is captured server-side and pushed back over the
      // SSE stream, which appends it (deduped) just like any other request.
      // So we only trigger the replay here and let the stream update the list,
      // avoiding a duplicate row.
      await apiReplayRequest(request.id)
    } catch (err) {
      console.error("replay failed", err)
    }
  }, [])

  return (
    <div className="flex min-h-screen bg-[var(--goport-bg)]">
      <Sidebar tunnelData={{ ...tunnel, requestsToday: requests.length }} />

      <main className="flex-1 flex flex-col overflow-hidden">
        <TopBar
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
          onClearRequests={handleClearRequests}
        />

        <RequestsTable requests={filteredRequests} onReplay={handleReplay} tunnelUrl={tunnel.url} />
      </main>
    </div>
  )
}
