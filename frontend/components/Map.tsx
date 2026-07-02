'use client'

import { useEffect, useLayoutEffect, useState, useCallback, useRef } from 'react'
import { MapContainer, TileLayer, Marker, Popup, Polyline, CircleMarker, useMap } from 'react-leaflet'
import L from 'leaflet'

const API = (process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080').replace(/\/+$/, '')
const KL_CENTER: [number, number] = [3.1390, 101.6869]

type Vehicle = {
  vehicle_id: string; lat: number; lon: number; bearing: number
  speed: number; delay_seconds: number; route_id: string; fetched_at: string
}

type Shape = {
  shape_id: string; route_id: string; points: { lat: number; lon: number }[]
}

type Route = {
  route_id: string; route_short_name: string; route_long_name: string
  route_color: string; route_type: number
}

type Station = {
  stop_id: string; stop_name: string; stop_lat: number; stop_lon: number
  route_ids: string[]; route_names: string[]; route_color: string
}

type ETA = {
  arrival_time: string; route_id: string; route_name: string
  route_color: string; trip_id: string; direction_id: number; headsign: string
}

type RouteLeg = {
  route_id: string; route_name: string; route_color: string
  direction_id: number; stops_between: number; duration_sec: number; shape_id: string
  from_stop: Station; to_stop: Station; stops?: string[]
}

type RoutePlanRoute = {
  route_id?: string; route_name?: string; route_color?: string
  direction_id?: number; stops_between?: number; duration_sec?: number; shape_id?: string
  legs: RouteLeg[]
  transfer_at?: Station
}

type RoutePlanResult = {
  routes: RoutePlanRoute[]; from_stop: Station; to_stop: Station
}

function delayColor(sec: number): string {
  if (sec < 120) return '#22c55e'
  if (sec < 300) return '#eab308'
  return '#ef4444'
}

function createIcon(color: string) {
  return L.divIcon({
    className: '',
    html: `<div style="width:12px;height:12px;background:${color};border-radius:50%;border:2px solid white;box-shadow:0 1px 3px rgba(0,0,0,.3)"></div>`,
    iconSize: [12, 12],
    iconAnchor: [6, 6],
  })
}

const userIcon = L.divIcon({
  className: '',
  html: `<div style="width:20px;height:20px;background:#2563eb;border-radius:50%;border:3px solid white;box-shadow:0 2px 6px rgba(0,0,0,.3);display:flex;align-items:center;justify-content:center"><div style="width:8px;height:8px;background:#fff;border-radius:50%"></div></div>`,
  iconSize: [20, 20],
  iconAnchor: [10, 10],
})

function VehicleMarker({ v }: { v: Vehicle }) {
  const color = delayColor(v.delay_seconds)
  return (
    <Marker position={[v.lat, v.lon]} icon={createIcon(color)}>
      <Popup>
        <b>{v.vehicle_id}</b><br />
        Route: {v.route_id || 'unknown'}<br />
        Delay: {v.delay_seconds > 0 ? `${Math.round(v.delay_seconds / 60)} min` : 'on time'}<br />
        Speed: {(v.speed || 0).toFixed(1)} km/h
      </Popup>
    </Marker>
  )
}

function ShapeLines({ shapes, routes, highlight }: { shapes: Shape[]; routes: Route[]; highlight?: string }) {
  const routeMap = new Map(routes.map(r => [r.route_id, r]))
  return (
    <>
      {shapes.map(s => {
        const isHL = highlight && s.route_id === highlight
        const color = routeMap.get(s.route_id)?.route_color
          ? `#${routeMap.get(s.route_id)!.route_color}`
          : '#666'
        return (
          <Polyline
            key={s.shape_id}
            positions={s.points.map(p => [p.lat, p.lon])}
            pathOptions={{ color, weight: isHL ? 6 : 4, opacity: isHL ? 1 : 0.7 }}
          />
        )
      })}
    </>
  )
}

function UserLocation() {
  const map = useMap()
  const [pos, setPos] = useState<[number, number] | null>(null)

  useEffect(() => {
    if (!navigator.geolocation) return
    navigator.geolocation.getCurrentPosition(
      p => {
        const c: [number, number] = [p.coords.latitude, p.coords.longitude]
        setPos(c)
        map.flyTo(c, 14, { duration: 2 })
      },
      () => {},
      { enableHighAccuracy: false, timeout: 8000 }
    )
  }, [map])

  if (!pos) return null
  return (
    <Marker position={pos} icon={userIcon}>
      <Popup>You are here</Popup>
    </Marker>
  )
}

function FlyTo({ pos }: { pos: [number, number] | null }) {
  const map = useMap()
  useEffect(() => {
    if (pos) map.flyTo(pos, 15, { duration: 1 })
  }, [map, pos])
  return null
}

function StationMarkers({ stations, onSelect }: { stations: Station[]; onSelect: (s: Station) => void }) {
  return (
    <>
      {stations.map(s => (
        <CircleMarker
          key={s.stop_id}
          center={[s.stop_lat, s.stop_lon]}
          radius={6}
          pathOptions={{
            color: s.route_color ? `#${s.route_color}` : '#666',
            fillColor: s.route_color ? `#${s.route_color}` : '#666',
            fillOpacity: 0.8,
            weight: 2,
          }}
          eventHandlers={{ click: () => onSelect(s) }}
        />
      ))}
    </>
  )
}

function fmtTime(t: string): string {
  const [h, m] = t.split(':').map(Number)
  const ap = h >= 12 ? 'PM' : 'AM'
  const h12 = h % 12 || 12
  return `${h12}:${String(m).padStart(2, '0')} ${ap}`
}

function minsUntil(arrival: string): string {
  const [h, m] = arrival.split(':').map(Number)
  const n = new Date()
  const myt = new Date(n.getTime() + n.getTimezoneOffset() * 60000 + 28800000)
  const d = h * 60 + m - (myt.getHours() * 60 + myt.getMinutes())
  if (d <= 0) return 'now'
  if (d < 60) return `${d} min`
  return `${Math.floor(d / 60)}h ${d % 60}m`
}

function StationPanel({ station, onClose }: { station: Station; onClose: () => void }) {
  const [etas, setEtas] = useState<ETA[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    fetch(`${API}/api/stations/${station.stop_id}/eta`)
      .then(r => r.json())
      .then(data => { setEtas(data); setLoading(false) })
      .catch(() => setLoading(false))
  }, [station.stop_id])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  const dirEtas = (dir: number) => etas.filter(e => e.direction_id === dir).slice(0, 3)

  return (
    <>
      <div
        onClick={onClose}
        style={{
          position: 'fixed', inset: 0, zIndex: 999, background: 'rgba(0,0,0,.3)',
        }}
      />
      <div style={{
        position: 'fixed', top: 10, right: 10, zIndex: 1000,
        background: 'rgba(255, 255, 255, 0.8)', borderRadius: 10, padding: 16, width: 300,
        boxShadow: '0 4px 16px rgba(0,0,0,.2)', fontFamily: 'system-ui, sans-serif',
        maxHeight: 'calc(100vh - 40px)', overflowY: 'auto', backdropFilter: 'blur(10px)', WebkitBackdropFilter: 'blur(10px)',
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 8 }}>
          <div>
            <div style={{ fontSize: 16, fontWeight: 700 }}>{station.stop_name}</div>
            <div style={{ fontSize: 12, color: '#666' }}>
              {station.route_names.slice(0, 3).join(' / ')}{station.route_names.length > 3 ? ' +more' : ''}
            </div>
          </div>
          <button onClick={onClose} style={{
            background: 'none', border: 'none', fontSize: 20, cursor: 'pointer',
            color: '#999', padding: '0 4px', lineHeight: 1,
          }}>×</button>
        </div>

        {loading ? (
          <div style={{ color: '#999', padding: '20px 0', textAlign: 'center', fontSize: 13 }}>Loading schedule...</div>
        ) : etas.length === 0 ? (
          <div style={{ color: '#999', padding: '20px 0', textAlign: 'center', fontSize: 13 }}>No upcoming arrivals</div>
        ) : (
          [0, 1].map(dir => {
            const items = dirEtas(dir)
            if (!items.length) return null
            return (
              <div key={dir} style={{ marginTop: dir === 1 ? 12 : 0 }}>
                <div style={{
                  fontWeight: 600, fontSize: 13, color: '#555', marginBottom: 6,
                  paddingBottom: 4, borderBottom: '1px solid #eee',
                }}>
                  {items[0]?.headsign || (dir === 0 ? 'Direction A' : 'Direction B')}
                </div>
                {items.map((e, i) => (
                  <div key={i} style={{
                    display: 'flex', alignItems: 'center', gap: 8, padding: '4px 0',
                  }}>
                    <span style={{
                      width: 10, height: 10, borderRadius: '50%', flexShrink: 0,
                      background: e.route_color ? `#${e.route_color}` : '#666',
                    }} />
                    <span style={{ flex: 1, fontSize: 13, color: '#333' }}>{e.route_name}</span>
                    <span style={{ fontWeight: 700, fontSize: 14 }}>{fmtTime(e.arrival_time)} <span style={{ fontWeight: 400, fontSize: 11, color: '#888' }}>({minsUntil(e.arrival_time)})</span></span>
                  </div>
                ))}
              </div>
            )
          })
        )}
      </div>
    </>
  )
}

function StationSearch({ stations, onSelect, placeholder }: { stations: Station[]; onSelect: (s: Station) => void; placeholder?: string }) {
  const [query, setQuery] = useState('')
  const [focused, setFocused] = useState(false)
  const [dropPos, setDropPos] = useState<{ top: number; left: number; width: number } | null>(null)
  const inputRef = useRef<HTMLDivElement>(null)
  const dropRef = useRef<HTMLDivElement>(null)

  const filtered = query.trim()
    ? stations.filter(s => s.stop_name.toLowerCase().includes(query.toLowerCase())).slice(0, 8)
    : []

  useLayoutEffect(() => {
    if (!focused || !inputRef.current) return
    inputRef.current.scrollIntoView({ block: 'nearest' })
    const r = inputRef.current.getBoundingClientRect()
    setDropPos({ top: r.bottom + 6, left: r.left, width: r.width })
  }, [focused, query])

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (inputRef.current && !inputRef.current.contains(e.target as Node) &&
          dropRef.current && !dropRef.current.contains(e.target as Node)) setFocused(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  return (
    <>
      <div style={{
        position: 'relative', flex: 1, width: '100%',
      }}>
        <div ref={inputRef} style={{ position: 'relative', flex: 1 }}>
          <div style={{
            display: 'flex', alignItems: 'center', gap: 8,
            background: focused ? 'rgba(255, 255, 255, 1)' : 'rgba(255, 255, 255, 0.7)', border: focused ? '1.5px solid #2563eb' : '1.5px solid transparent',
            borderRadius: 10, padding: '0 12px', backdropFilter: 'blur(10px)', WebkitBackdropFilter: 'blur(10px)',
            boxShadow: focused
              ? '0 4px 20px rgba(37,99,235,.15), 0 1px 3px rgba(0,0,0,.08)'
              : '0 2px 8px rgba(0,0,0,.08)',
            transition: 'box-shadow .15s, border .15s',
          }}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={focused ? '#2563eb' : '#999'} strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0 }}>
              <circle cx="11" cy="11" r="8" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
            <input
              placeholder={placeholder || 'Search stations...'}
              value={query}
              onChange={e => { setQuery(e.target.value); setFocused(true) }}
              onFocus={() => setFocused(true)}
              style={{
                flex: 1, border: 'none', outline: 'none', fontSize: 14, padding: '10px 0',
                fontFamily: 'system-ui, sans-serif', color: '#1a1a1a',
                background: 'transparent',
              }}
            />
            {query && (
              <button
                onClick={() => setQuery('')}
                style={{
                  background: '#e5e7eb', border: 'none', borderRadius: '50%', cursor: 'pointer',
                  width: 18, height: 18, display: 'flex', alignItems: 'center', justifyContent: 'center',
                  padding: 0, flexShrink: 0, color: '#666', fontSize: 12, lineHeight: 1,
                }}
              >✕</button>
            )}
          </div>
        </div>

        {dropPos && focused && query.length > 0 && filtered.length > 0 && (
          <div ref={dropRef} style={{
            position: 'fixed', top: dropPos.top, left: dropPos.left, width: dropPos.width, zIndex: 9999,
            background: 'rgba(255, 255, 255, 0.8)', borderRadius: 10,
            boxShadow: '0 8px 30px rgba(0,0,0,.12)',
            border: '1px solid #f0f0f0', backdropFilter: 'blur(10px)', WebkitBackdropFilter: 'blur(10px)',
          }}>
            <div style={{
              maskImage: "linear-gradient(to bottom, transparent 0px, black 30px, black calc(100% - 20px), transparent 100%)",
              WebkitMaskImage: "linear-gradient(to bottom, transparent 0px, black 30px, black calc(100% - 30px), transparent 100%)",
              scrollbarWidth: 'none', overflow: 'auto', height: 260, paddingBottom: 5, paddingTop: 5,
              background: 'transparent',
            }}>
              {filtered.map(s => {
                const color = s.route_color ? `#${s.route_color}` : '#999'
                return (
                  <button
                    key={s.stop_id}
                    onClick={() => { onSelect(s); setQuery(s.stop_name); setFocused(false) }}
                    style={{
                      display: 'flex', alignItems: 'center', gap: 10, width: '100%',
                      padding: '10px 14px', border: 'none', borderBottom: '1px solid #f5f5f5',
                      textAlign: 'left', cursor: 'pointer', fontSize: 13, background: 'white',
                      fontFamily: 'system-ui, sans-serif', transition: 'background .1s',
                    }}
                    onMouseEnter={e => (e.currentTarget.style.background = '#f8f9ff')}
                    onMouseLeave={e => (e.currentTarget.style.background = 'white')}
                  >
                    <div style={{
                      width: 10, height: 10, borderRadius: '50%', flexShrink: 0, background: color,
                      border: `2px solid ${color}33`,
                    }} />
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontWeight: 600, color: '#1a1a1a', fontSize: 13 }}>{s.stop_name}</div>
                      {s.route_names.length > 0 && (
                        <div style={{ color: '#888', fontSize: 11, marginTop: 2 }}>
                          {s.route_names.slice(0, 2).join(' · ')}
                        </div>
                      )}
                    </div>
                    <span style={{ color: '#bbb', fontSize: 11, fontFamily: 'monospace' }}>{s.stop_id}</span>
                  </button>
                )
              })}
            </div>
          </div>
        )}
      </div>
    </>
  )
}

function RoutePlanner({ stations, onRouteFound, onClose }: {
  stations: Station[]; onRouteFound: (from: Station, to: Station, routes: RoutePlanRoute[]) => void; onClose: () => void
}) {
  const [from, setFrom] = useState<Station | null>(null)
  const [to, setTo] = useState<Station | null>(null)
  const [loading, setLoading] = useState(false)
  const [results, setResults] = useState<RoutePlanRoute[] | null>(null)
  const [error, setError] = useState('')
  const [expandedIdx, setExpandedIdx] = useState<number | null>(null)

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  const findRoute = async () => {
    if (!from || !to) return
    if (from.stop_id === to.stop_id) { setError('Start and end must be different'); return }
    setLoading(true); setError(''); setResults(null)
    try {
      const res = await fetch(`${API}/api/route-plan?from=${from.stop_id}&to=${to.stop_id}`)
      if (!res.ok) { setError('No routes found'); setLoading(false); return }
      const data: RoutePlanResult = await res.json()
      if (data.routes.length === 0) { setError('No direct route between these stations'); setLoading(false); return }
      setResults(data.routes)
      onRouteFound(data.from_stop, data.to_stop, data.routes)
    } catch { setError('Failed to find route') }
    setLoading(false)
  }

  const swap = () => { setFrom(to); setTo(from); setResults(null); setError('') }
  const ready = from && to

  return (
    <>
      <div onClick={onClose} style={{ position: 'fixed', inset: 0, zIndex: 998, background: 'rgba(0,0,0,.3)' }} />
      <div style={{
        width: '100%', background: 'rgba(255,255,255,.70)', borderRadius: 12, padding: 16,
        boxShadow: '0 8px 30px rgba(0,0,0,.12)', fontFamily: 'system-ui, sans-serif',
        position: 'relative', zIndex: 999, backdropFilter: 'blur(10px)', WebkitBackdropFilter: 'blur(10px)',
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 14 }}>
          <span style={{ fontWeight: 700, fontSize: 14, color: '#1a1a1a' }}>Plan a route</span>
          <button onClick={onClose} style={{
            background: '#f1f3f5', border: 'none', borderRadius: 8,
            width: 28, height: 28, cursor: 'pointer', color: '#666',
            fontSize: 15, lineHeight: 1, display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>✕</button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <div style={{
              width: 28, height: 28, borderRadius: 8, background: '#e8f4e8',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              flexShrink: 0, color: '#16a34a', fontSize: 14, fontWeight: 700,
            }}>A</div>
            <StationSearch stations={stations} onSelect={setFrom} placeholder="From..." />
            <button onClick={swap} style={{
              width: 28, height: 28, borderRadius: 8, background: '#f1f3f5',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              border: 'none', cursor: 'pointer', flexShrink: 0,
              fontSize: 14, color: '#666',
            }}>⇄</button>
          </div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <div style={{
              width: 28, height: 28, borderRadius: 8, background: '#fef2f2',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              flexShrink: 0, color: '#dc2626', fontSize: 14, fontWeight: 700,
            }}>B</div>
            <StationSearch stations={stations} onSelect={setTo} placeholder="To..." />
          </div>
        </div>

        <button onClick={findRoute} disabled={!ready || loading} style={{
          width: '100%', padding: '10px', border: 'none', borderRadius: 10, marginTop: 12,
          background: ready ? '#2563eb' : '#e5e7eb', color: ready ? 'white' : '#999',
          fontSize: 14, fontWeight: 600, cursor: ready ? 'pointer' : 'default',
          transition: 'all .15s', letterSpacing: '.01em',
        }}>
          {loading ? 'Searching...' : ready ? `Find route from ${from.stop_name} to ${to.stop_name}` : 'Select two stations'}
        </button>

        {error && <div style={{
          color: '#dc2626', fontSize: 13, marginTop: 10, textAlign: 'center',
          padding: '8px 12px', background: '#fef2f2', borderRadius: 8,
        }}>{error}</div>}

        {results && results.length > 0 && (
          <div style={{ marginTop: 14 }}>
            <div style={{ fontSize: 12, color: '#888', marginBottom: 8, fontWeight: 500 }}>
              {results.length} route{results.length > 1 ? 's' : ''} found
            </div>
            {results.map((r, i) => {
              const expanded = expandedIdx === i
              const color = r.legs[0]?.route_color || 'ccc'
              const totalStops = r.legs.reduce((s, l) => s + (l.stops?.length || 0), 0)
              return (
              <div key={i} style={{ marginBottom: 6 }}>
                <button
                  onClick={() => setExpandedIdx(expanded ? null : i)}
                  style={{
                    width: '100%', padding: '10px 12px', borderRadius: 8, border: 'none',
                    textAlign: 'left', cursor: 'pointer', fontFamily: 'system-ui, sans-serif',
                    background: `#${color}08`, borderLeft: `3px solid #${color}`,
                    display: 'flex', alignItems: 'center', gap: 8,
                    transition: 'background .1s',
                  }}
                >
                  <div style={{
                    width: 10, height: 10, borderRadius: '50%', flexShrink: 0,
                    background: `#${color}`, border: `2px solid #${color}44`,
                  }} />
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontWeight: 600, fontSize: 13, color: '#1a1a1a' }}>
                      {r.legs.length === 1 ? r.legs[0].route_name : `${r.legs[0].route_name} → ${r.legs[1].route_name}`}
                    </div>
                    <div style={{ fontSize: 11, color: '#888', marginTop: 2 }}>
                      {r.legs[0].from_stop.stop_name} → {r.legs[r.legs.length-1].to_stop.stop_name}
                      {totalStops > 0 && ` · ${totalStops} stop${totalStops !== 1 ? 's' : ''}`}
                    </div>
                  </div>
                  <span style={{ color: '#999', fontSize: 11, transition: 'transform .15s', transform: expanded ? 'rotate(180deg)' : 'none' }}>▼</span>
                </button>

                {expanded && (
                  <div style={{
                    marginTop: 4, padding: '6px 12px 10px 12px',
                    background: '#fafafa', borderRadius: 8,
                    border: '1px solid #f0f0f0',
                  }}>
                    {r.legs.map((leg, li) => (
                      <div key={li}>
                        {li > 0 && (
                          <div style={{
                            display: 'flex', alignItems: 'center', gap: 6, padding: '6px 0',
                            fontSize: 11, color: '#2563eb', fontWeight: 600,
                          }}>
                            <span style={{ flex: 1, height: 1, background: '#dbeafe' }} />
                            Transfer at {r.transfer_at?.stop_name}
                            <span style={{ flex: 1, height: 1, background: '#dbeafe' }} />
                          </div>
                        )}
                        <div>
                          <div style={{ fontSize: 11, color: '#888', marginBottom: 4, fontWeight: 600 }}>
                            <span style={{ color: `#${leg.route_color}` }}>●</span> {leg.route_name}
                            <span style={{ fontWeight: 400, color: '#aaa' }}> · {leg.stops?.length} stops</span>
                          </div>
                          <div style={{ position: 'relative' }}>
                            {(leg.stops || []).map((name, si) => (
                              <div key={si} style={{
                                display: 'flex', alignItems: 'center', gap: 8,
                                padding: '3px 0', position: 'relative',
                              }}>
                                <div style={{
                                  width: 12, height: 12, borderRadius: '50%', flexShrink: 0, zIndex: 1,
                                  background: si === 0 || si === (leg.stops?.length || 0) - 1
                                    ? `#${leg.route_color}` : 'white',
                                  border: `2px solid #${leg.route_color}`,
                                }} />
                                <div style={{
                                  fontSize: 12, color: '#1a1a1a', fontWeight: si === 0 || si === (leg.stops?.length || 0) - 1 ? 600 : 400,
                                }}>
                                  {name}
                                  {si === 0 && <span style={{ color: '#888', fontWeight: 400, marginLeft: 4, fontSize: 10 }}>(start)</span>}
                                  {si === (leg.stops?.length || 0) - 1 && <span style={{ color: li < r.legs.length - 1 ? '#2563eb' : '#16a34a', fontWeight: 400, marginLeft: 4, fontSize: 10 }}>({li < r.legs.length - 1 ? 'transfer' : 'end'})</span>}
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )})}
          </div>
        )}
      </div>
    </>
  )
}

export function TransitMap() {
  const [vehicles, setVehicles] = useState<Vehicle[]>([])
  const [shapes, setShapes] = useState<Shape[]>([])
  const [routes, setRoutes] = useState<Route[]>([])
  const [stations, setStations] = useState<Station[]>([])
  const [selectedStation, setSelectedStation] = useState<Station | null>(null)
  const [showRoutePlanner, setShowRoutePlanner] = useState(false)
  const [highlightRoute, setHighlightRoute] = useState<string | undefined>()
  const [flyPos, setFlyPos] = useState<[number, number] | null>(null)

  const fetchVehicles = useCallback(async () => {
    try {
      const res = await fetch(`${API}/api/vehicles`)
      if (!res.ok) return
      const data = await res.json()
      setVehicles(data.vehicles || [])
    } catch {}
  }, [])

  useEffect(() => {
    fetchVehicles()
    const t = setInterval(fetchVehicles, 30000)
    return () => clearInterval(t)
  }, [fetchVehicles])

  useEffect(() => {
    fetch(`${API}/api/shapes`).then(r => r.json()).then(setShapes).catch(() => {})
    fetch(`${API}/api/routes`).then(r => r.json()).then(setRoutes).catch(() => {})
    fetch(`${API}/api/stations`).then(r => r.json()).then(setStations).catch(() => {})
  }, [])

  const handleStationClick = (s: Station) => {
    setSelectedStation(s)
    setShowRoutePlanner(false)
    setFlyPos([s.stop_lat, s.stop_lon])
  }

  const handleRouteFound = (from: Station, to: Station, plan: RoutePlanRoute[]) => {
    if (plan.length > 0) {
      setHighlightRoute(plan[0].legs[0]?.route_id)
      const mid = { lat: (from.stop_lat + to.stop_lat) / 2, lon: (from.stop_lon + to.stop_lon) / 2 }
      setFlyPos([mid.lat, mid.lon])
    }
  }

  const handleRouteSelect = (routeId: string) => {
    setHighlightRoute(routeId === highlightRoute ? undefined : routeId)
  }

  return (
    <div style={{ position: 'relative', height: '100%', width: '100%' }}>
      {!selectedStation && <div style={{
        position: 'absolute', top: 12, right: 12,
        zIndex: 1000, display: 'flex', flexDirection: 'column', gap: 8,
        width: 320, maxWidth: 'calc(100vw - 40px)',
        maxHeight: 'calc(100vh - 60px)', overflowY: 'auto',
      }}>
        <div style={{
          display: 'flex', gap: 2, padding: 3, background: '#f1f3f5cb', borderRadius: 10, position: 'sticky', top: 0, zIndex: 10000, backdropFilter: 'blur(10px)', 
        }}>
          {[
            { label: 'Stations', active: !showRoutePlanner, onClick: () => { setShowRoutePlanner(false); setHighlightRoute(undefined) } },
            { label: 'Routes', active: showRoutePlanner, onClick: () => { setShowRoutePlanner(true); setSelectedStation(null) } },
          ].map(b => (
            <button key={b.label} onClick={b.onClick} style={{
              padding: '5px 16px', border: 'none', cursor: 'pointer', fontSize: 12, display: 'flex',
              fontWeight: 600, fontFamily: 'system-ui, sans-serif',
              borderRadius: 7, letterSpacing: '.02em', alignItems: 'center',
              background: b.active ? 'white' : 'transparent',
              color: b.active ? '#1a1a1a' : '#888',
              boxShadow: b.active ? '0 1px 3px rgba(0,0,0,.08)' : 'none',
              transition: 'all .15s',
            }}>
              <span style={{ marginRight: 5, height: 24 }}>
                <svg xmlns="http://www.w3.org/2000/svg" height="24px" viewBox="0 -960 960 960" width="24px" fill={b.active ? '#1a1a1a' : '#888'}><path d={b.label === 'Stations' ? "M80-80v-526q0-85 44-147.5T248-848q54-21 115-26.5t117-5.5q56 0 117 5.5T712-848q80 32 124 94.5T880-606v526H80Zm284-80h230l-60-60H424l-60 60Zm-64-280h360v-160H300v160Zm348.5 128.5Q660-323 660-340t-11.5-28.5Q637-380 620-380t-28.5 11.5Q580-357 580-340t11.5 28.5Q603-300 620-300t28.5-11.5Zm-280 0Q380-323 380-340t-11.5-28.5Q357-380 340-380t-28.5 11.5Q300-357 300-340t11.5 28.5Q323-300 340-300t28.5-11.5ZM160-160h140v-20l42-42q-44-6-73-39.5T240-340v-260q0-78 74.5-99T480-720q100 0 170 21t70 99v260q0 45-29 78.5T618-222l42 42v20h140v-446q0-60-29.5-102.5T682-774q-44-17-97.5-21.5T480-800q-51 0-104.5 4.5T278-774q-59 23-88.5 65.5T160-606v446Zm0 0h640-640Z" : "M247-167q-47-47-47-113v-327q-35-13-57.5-43.5T120-720q0-50 35-85t85-35q50 0 85 35t35 85q0 39-22.5 69.5T280-607v327q0 33 23.5 56.5T360-200q33 0 56.5-23.5T440-280v-400q0-66 47-113t113-47q66 0 113 47t47 113v327q35 13 57.5 43.5T840-240q0 50-35 85t-85 35q-50 0-85-35t-35-85q0-39 22.5-70t57.5-43v-327q0-33-23.5-56.5T600-760q-33 0-56.5 23.5T520-680v400q0 66-47 113t-113 47q-66 0-113-47Zm-7-513q17 0 28.5-11.5T280-720q0-17-11.5-28.5T240-760q-17 0-28.5 11.5T200-720q0 17 11.5 28.5T240-680Zm480 480q17 0 28.5-11.5T760-240q0-17-11.5-28.5T720-280q-17 0-28.5 11.5T680-240q0 17 11.5 28.5T720-200ZM240-720Zm480 480Z"} /></svg>
              </span> {b.label}
            </button>
          ))}
        </div>

        {!showRoutePlanner ? (
          <StationSearch
            stations={stations}
            onSelect={handleStationClick}
            placeholder="Search stations..."
          />
        ) : (
          <RoutePlanner
            stations={stations}
            onRouteFound={handleRouteFound}
            onClose={() => setShowRoutePlanner(false)}
          />
        )}
      </div>}

      <MapContainer attributionControl={false} center={KL_CENTER} zoom={12} style={{ height: '100%', width: '100%' }}>
        <TileLayer
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          attribution="&copy; OpenStreetMap contributors"
        />
        <ShapeLines shapes={shapes} routes={routes} highlight={highlightRoute} />
        <StationMarkers stations={stations} onSelect={handleStationClick} />
        {vehicles.map(v => <VehicleMarker key={v.vehicle_id} v={v} />)}
        <UserLocation />
        <FlyTo pos={flyPos} />
      </MapContainer>
      {selectedStation && <StationPanel station={selectedStation} onClose={() => setSelectedStation(null)} />}
    </div>
  )
}
