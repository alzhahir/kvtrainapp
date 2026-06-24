'use client'

import { useEffect, useState } from 'react'

const API = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

type Status = {
  line_id: string; line_name: string; status: string; remarks: string
}

function statusColor(s: string): string {
  switch (s) {
    case 'Normal': return '#22c55e'
    case 'Delay': return '#eab308'
    case 'Suspended': return '#ef4444'
    default: return '#6b7280'
  }
}

export function ServiceBar() {
  const [statuses, setStatuses] = useState<Status[]>([])

  useEffect(() => {
    const fetchStatus = async () => {
      try {
        const res = await fetch(`${API}/api/service-status`)
        if (!res.ok) return
        const data = await res.json()
        setStatuses(data.statuses || [])
      } catch {}
    }
    fetchStatus()
    const t = setInterval(fetchStatus, 60000)
    return () => clearInterval(t)
  }, [])

  if (statuses.length === 0) return null

  return (
    <div style={{
      position: 'absolute', top: 10, left: 10, zIndex: 1000,
      background: 'white', borderRadius: 8, padding: 12,
      boxShadow: '0 2px 8px rgba(0,0,0,.15)', maxWidth: 280,
      fontFamily: 'system-ui, sans-serif', fontSize: 13,
    }}>
      <h3 style={{ margin: '0 0 8px', fontSize: 14, fontWeight: 600 }}>
        Service Status
      </h3>
      {statuses.map(s => (
        <div key={s.line_id} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
          <span style={{
            width: 10, height: 10, borderRadius: '50%',
            background: statusColor(s.status), flexShrink: 0,
          }} />
          <span style={{ flex: 1, fontWeight: 500 }}>{s.line_name}</span>
          <span style={{ color: statusColor(s.status), fontSize: 12 }}>{s.status}</span>
        </div>
      ))}
    </div>
  )
}
