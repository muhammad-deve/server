"use client"

import { HttpMethod } from "@/lib/types"

interface MethodBadgeProps {
  method: HttpMethod
}

const methodConfig: Record<HttpMethod, string> = {
  GET: "bg-[#00d4ff]/10 text-[#00d4ff]",
  POST: "bg-[#8b5cf6]/10 text-[#8b5cf6]",
  PUT: "bg-[#eab308]/10 text-[#eab308]",
  DELETE: "bg-[#ef4444]/10 text-[#ef4444]",
  PATCH: "bg-[#f97316]/10 text-[#f97316]",
}

export function MethodBadge({ method }: MethodBadgeProps) {
  const colorClass = methodConfig[method]
  
  return (
    <span className={`inline-flex items-center justify-center w-14 px-2 py-0.5 rounded text-xs font-medium font-mono ${colorClass}`}>
      {method}
    </span>
  )
}

interface StatusBadgeProps {
  statusCode: number
}

export function StatusBadge({ statusCode }: StatusBadgeProps) {
  let colorClass = ""
  
  if (statusCode >= 200 && statusCode < 300) {
    colorClass = "bg-[#22c55e]/10 text-[#22c55e]"
  } else if (statusCode >= 400 && statusCode < 500) {
    colorClass = "bg-[#eab308]/10 text-[#eab308]"
  } else if (statusCode >= 500) {
    colorClass = "bg-[#ef4444]/10 text-[#ef4444]"
  } else {
    colorClass = "bg-[#64748b]/10 text-[#64748b]"
  }
  
  return (
    <span className={`inline-flex items-center justify-center px-2 py-0.5 rounded text-xs font-medium font-mono ${colorClass}`}>
      {statusCode}
    </span>
  )
}
