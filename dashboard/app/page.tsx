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

  // Live stream new requests via SSE.
  useEffect(() => {
    const teardown = subscribeRequests((req) => {
      setRequests((prev) => {
        if (prev.find((r) => r.id === req.id)) return prev
        return [req, ...prev].slice(0, 500)
      })
      setTunnel((prev) => ({ ...prev, requestsToday: prev.requestsToday + 1 }))
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
      const replayed = await apiReplayRequest(request.id)
      setRequests((prev) => [replayed, ...prev].slice(0, 500))
    } catch (err) {
      console.error("replay failed", err)
    }
  }, [])

  return (
    <div className="flex min-h-screen bg-[var(--goport-bg)]">
      <Sidebar tunnelData={tunnel} />

      <main className="flex-1 flex flex-col overflow-hidden">
        <TopBar
          tunnelUrl={tunnel.url}
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
          onClearRequests={handleClearRequests}
          requestCount={filteredRequests.length}
        />

        <RequestsTable requests={filteredRequests} onReplay={handleReplay} tunnelUrl={tunnel.url} />
      </main>
    </div>
  )
}
