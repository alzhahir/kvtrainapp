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
      <footer style={{
        display: "flex", position: "absolute", bottom: 0, width: "100%", padding: 4, zIndex: 1000,
        justifyContent: "space-between", alignItems: "center"
      }}>
        <div style={{
          background: 'rgba(255,255,255,.85)', padding: '3px 8px', borderRadius: 5,
          fontSize: 12, color: '#666',
        }}>
          Data from data.gov.my &bull; Updated every 30s
        </div>
        <div style={{
          position: 'relative'
        }}>
          <button popoverTarget='mapattrib' title='Attribution' style={{
            background: 'rgba(255,255,255,.85)', padding: '3px 8px', borderRadius: 5,
            fontSize: 16, color: '#666',
            anchorName: '--attribpopover',
            borderWidth: 0, display: 'flex', alignItems: 'center',
          }}>
            <svg xmlns="http://www.w3.org/2000/svg" height="16px" viewBox="0 -960 960 960" width="16px" fill="#666"><path d="M430-200h100v-180h60v-184q0-27-28.5-41.5T480-620q-53 0-81.5 14.5T370-564v184h60v180Zm-105 88.5q-73-31.5-127.5-86t-86-127.5Q80-398 80-480.5t31.5-155q31.5-72.5 86-127t127.5-86Q398-880 480.5-880t155 31.5q72.5 31.5 127 86t86 127Q880-563 880-480.5T848.5-325q-31.5 73-86 127.5t-127 86Q563-80 480.5-80T325-111.5Zm381.5-142Q800-347 800-480t-93.5-226.5Q613-800 480-800t-226.5 93.5Q160-613 160-480t93.5 226.5Q347-160 480-160t226.5-93.5ZM523-657q17-17 17-43t-17-43q-17-17-43-17t-43 17q-17 17-17 43t17 43q17 17 43 17t43-17Zm-43 177Z"/></svg>
          </button>
          <div popover='auto' id='mapattrib' style={{
            left: 4,
            background: 'rgba(255,255,255,.85)', padding: '3px 8px', borderRadius: 5,
            fontSize: 12, color: '#666',
            positionAnchor: '--attribpopover', positionArea: 'center left',
            borderWidth: 0,
          }}>
            &copy; {new Date().getFullYear()} Ierultronic &bull; Powered by <a style={{textDecoration:"none"}} target="_blank" href="https://leafletjs.com">Leaflet</a> &bull; Map data &copy; <a style={{textDecoration:"none"}} target="_blank" href="https://www.openstreetmap.org/copyright">OpenStreetMap Contributors</a>
          </div>
        </div>
      </footer>
    </div>
  )
}
