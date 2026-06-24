'use client'

import dynamic from 'next/dynamic'
import { ServiceBar } from '../components/ServiceBar'

const TransitMap = dynamic(() => import('../components/Map').then(m => ({ default: m.TransitMap })), {
  ssr: false,
})

export default function Home() {
  return (
    <div style={{ height: '100vh', width: '100vw', position: 'relative' }}>
      <TransitMap />
      <ServiceBar />
      <div style={{
        position: 'absolute', bottom: 10, right: 10, zIndex: 1000,
        background: 'white', padding: '4px 10px', borderRadius: 6,
        fontSize: 12, boxShadow: '0 1px 4px rgba(0,0,0,.15)',
      }}>
        Data from data.gov.my &bull; Updated every 30s
      </div>
    </div>
  )
}
