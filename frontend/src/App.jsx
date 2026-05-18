import { useState, useEffect, useRef, useCallback } from 'react'
import './index.css'
import NetworkGraph from './NetworkGraph'
import JarvisAssistant from './JarvisAssistant'

const isDev = window.location.port === '5173'
const API_URL = isDev ? 'http://localhost:8080/api' : '/api'

function BrandLogo({ className = '', size = 32, color = '#00ff66' }) {
  return (
    <svg 
      width={size} 
      height={size} 
      viewBox="0 0 100 100" 
      fill="none" 
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      style={{ display: 'inline-block', verticalAlign: 'middle' }}
    >
      <defs>
        <filter id="logoGlow" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="3" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>
      
      {/* Outer Hexagonal Shield Outline */}
      <path
        d="M50 8 L85 24 L85 54 C85 74 50 92 50 92 C50 92 15 74 15 54 L15 24 Z"
        stroke={color}
        strokeWidth="6"
        strokeLinecap="round"
        strokeLinejoin="round"
        filter="url(#logoGlow)"
      />
      
      {/* Inner Stylized Geometric 'S' */}
      <path
        d="M30 38 L50 28 L70 38 L70 48 L32 58 C30 59 30 63 32 64 L50 72 L70 64"
        stroke={color}
        strokeWidth="6"
        strokeLinecap="round"
        strokeLinejoin="round"
        filter="url(#logoGlow)"
      />
    </svg>
  );
}

const SEVERITY_ORDER = { critical: 0, high: 1, medium: 2, low: 3 }

function formatTime(d) {
  const dt = d ? new Date(d) : new Date()
  return dt.toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function SeverityBadge({ severity }) {
  return <span className={`alert-severity-badge ${severity}`}>{severity}</span>
}

/* ── OVERLAY PANELS ── */

function AlertsPanel({ alerts }) {
  return (
    <div className="overlay-panel">
      <div className="overlay-panel-title">Alerts ({alerts.length})</div>
      {alerts.length === 0 ? (
        <div className="overlay-empty">No alerts yet</div>
      ) : (
        <div className="overlay-scroll">
          {[...alerts].sort((a,b) => (SEVERITY_ORDER[a.severity]??9)-(SEVERITY_ORDER[b.severity]??9)).slice(0, 50).map((a, i) => (
            <div className="alert-row" key={i}>
              <SeverityBadge severity={a.severity} />
              <div className="alert-row-content">
                <div className="alert-row-msg">{a.message}</div>
                <div className="alert-row-meta">
                  {a.source_ip && <span>IP: {a.source_ip}</span>}
                  <span className="alert-type-tag">{a.type}</span>
                  <span>{formatTime(a.timestamp)}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function UptimePanel({ data }) {
  return (
    <div className="overlay-panel">
      <div className="overlay-panel-title">Service Uptime</div>
      {data.length === 0 ? (
        <div className="overlay-empty">Waiting for data...</div>
      ) : (
        <div className="overlay-scroll">
          {data.map((u, i) => (
            <div className="uptime-row" key={i}>
              <span className="uptime-name">{u.service}</span>
              <span className={`uptime-status ${u.status}`}>
                <span className="uptime-dot" /> {u.status.toUpperCase()}
              </span>
              <span className="uptime-latency">{u.latency}ms</span>
              <span className="uptime-pct">{u.uptime_percent?.toFixed(1)}%</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function ControlPanel({ actions }) {
  return (
    <div className="overlay-panel">
      <div className="overlay-panel-title">Blocked IPs</div>
      {(!actions.blocked_ips || actions.blocked_ips.length === 0) ? (
        <div className="overlay-empty">No blocked IPs</div>
      ) : (
        <div className="overlay-scroll">
          {actions.blocked_ips.map((ip, i) => (
            <div className="blocked-row" key={i}>
              <span className="blocked-ip">{ip}</span>
              <span className="blocked-badge">BLOCKED</span>
              <span className="blocked-source">J.A.R.V.I.S</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function LogPanel({ logs }) {
  const ref = useRef(null)
  useEffect(() => { if (ref.current) ref.current.scrollTop = ref.current.scrollHeight }, [logs])
  return (
    <div className="overlay-panel">
      <div className="overlay-panel-title">Live Logs ({logs.length})</div>
      <div className="overlay-scroll log-scroll" ref={ref}>
        {logs.length === 0 ? (
          <div className="overlay-empty">Waiting for logs...</div>
        ) : logs.slice(-100).map((l, i) => (
          <div className="log-line" key={i}>
            <span className="log-time">{formatTime(l.ts)}</span>
            <span className={`log-source ${l.source}`}>{l.source}</span>
            <span className="log-text">{l.raw_log}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function StatsPanel({ stats, alerts }) {
  const crit = stats.by_severity?.critical || 0
  const high = stats.by_severity?.high || 0
  const med  = stats.by_severity?.medium || 0
  const total = stats.total_logs || 0

  return (
    <div className="overlay-panel">
      <div className="overlay-panel-title">Statistics</div>
      <div className="stats-mini-grid">
        <div className="stat-mini critical"><span className="stat-mini-val">{crit}</span><span className="stat-mini-lbl">Critical</span></div>
        <div className="stat-mini high"><span className="stat-mini-val">{high}</span><span className="stat-mini-lbl">High</span></div>
        <div className="stat-mini medium"><span className="stat-mini-val">{med}</span><span className="stat-mini-lbl">Medium</span></div>
        <div className="stat-mini ok"><span className="stat-mini-val">{total}</span><span className="stat-mini-lbl">Total Logs</span></div>
      </div>
      <div className="overlay-panel-title" style={{marginTop:16}}>Distribution</div>
      {[
        { key: 'brute_force', label: 'Brute Force', color: '#ef4444' },
        { key: 'unauthorized_access', label: 'Unauthorized', color: '#f97316' },
        { key: 'sqli_attempt', label: 'SQL Injection', color: '#8b5cf6' },
        { key: 'port_scan', label: 'Port Scan', color: '#06b6d4' },
      ].map(t => {
        const count = alerts.filter(a => a.type === t.key).length
        const max = Math.max(alerts.length, 1)
        return (
          <div className="dist-row" key={t.key}>
            <span className="dist-label">{t.label}</span>
            <div className="dist-bar-bg"><div className="dist-bar" style={{width:`${(count/max)*100}%`, background:t.color}} /></div>
            <span className="dist-count">{count}</span>
          </div>
        )
      })}
    </div>
  )
}

function DiscoveryPanel({ apiKey }) {
  const [dirs, setDirs] = useState('/Users/niyazimertisiksal/Concurrent-Log-Streamer');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState(null);
  const [codeOnly, setCodeOnly] = useState(false);

  const handleScan = async () => {
    setLoading(true);
    setResult(null);
    try {
      const dirArray = dirs.split(',').map(d => d.trim()).filter(d => d);
      const res = await fetch(`${API_URL}/discover`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-API-Key': apiKey },
        body: JSON.stringify({ dirs: dirArray, code_only: codeOnly })
      });
      const data = await res.json();
      setResult(data);
    } catch (err) {
      setResult({ error: err.message });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="overlay-panel">
      <div className="overlay-panel-title">Auto-Discovery Agent</div>
      <div style={{ marginBottom: 15, fontSize: 13, color: 'var(--text-secondary)' }}>
        Enter target project directory paths on your local machine (comma separated) to run static code analysis.
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', marginBottom: 15 }}>
        <input
          type="text"
          className="discovery-input"
          style={{
            background: 'rgba(15,23,42,0.8)',
            border: '1px solid var(--border-bright)',
            padding: '10px 14px',
            borderRadius: '6px',
            color: '#fff',
            fontSize: '13px',
            fontFamily: 'var(--font-mono)'
          }}
          value={dirs}
          onChange={(e) => setDirs(e.target.value)}
        />
        <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', color: 'var(--text-secondary)' }}>
          <input
            type="checkbox"
            checked={codeOnly}
            onChange={(e) => setCodeOnly(e.target.checked)}
          />
          Static analysis only (skip active container scan)
        </label>
      </div>
      <button
        onClick={handleScan}
        disabled={loading}
        className="scan-btn"
        style={{
          background: 'linear-gradient(135deg, #3b82f6 0%, #2563eb 100%)',
          color: '#fff',
          padding: '10px 20px',
          border: 'none',
          borderRadius: '6px',
          fontWeight: '600',
          cursor: loading ? 'not-allowed' : 'pointer',
          opacity: loading ? 0.7 : 1,
          boxShadow: '0 4px 15px rgba(59, 130, 246, 0.3)'
        }}
      >
        {loading ? 'Scanning & Discovering Infrastructure...' : 'Discover & Scan Directories'}
      </button>

      {result && (
        <div className="discovery-results" style={{ marginTop: 20 }}>
          <div className="overlay-panel-title" style={{ fontSize: '14px', marginBottom: 10 }}>Scan Reports</div>
          {result.error ? (
            <div style={{ color: 'var(--red)', background: 'rgba(239, 68, 68, 0.08)', padding: 12, borderRadius: 6, border: '1px dashed rgba(239,68,68,0.3)' }}>
              Scan failed: {result.error}
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              <div style={{ background: 'rgba(34, 197, 94, 0.08)', color: '#4ade80', padding: 12, borderRadius: 6, border: '1px solid rgba(34, 197, 94, 0.2)', fontSize: 13 }}>
                ✓ {result.message || 'Auto-discovery completed successfully! Created topology representation.'}
              </div>
              <div style={{ background: 'rgba(15,23,42,0.6)', border: '1px solid var(--border)', padding: 15, borderRadius: 8 }}>
                <div style={{ fontSize: 12, fontWeight: 700, marginBottom: 8, color: 'var(--text-secondary)' }}>INFRASTRUCTURE OVERVIEW</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
                  <div style={{ fontSize: 13 }}>Nodes Discovered: <strong style={{ color: 'var(--accent-cyan)' }}>{result.stats?.nodes_discovered || 0}</strong></div>
                  <div style={{ fontSize: 13 }}>Service Links: <strong style={{ color: 'var(--orange)' }}>{result.stats?.links_discovered || 0}</strong></div>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/* ── NEXT-GEN SAAS COMPONENTS ── */

function LandingPage({ onLogin }) {
  const containerRef = useRef(null)
  const canvasRef = useRef(null)
  const [scrollFraction, setScrollFraction] = useState(0)
  const [offsetCounter, setOffsetCounter] = useState(0)
  const [cpuPulse, setCpuPulse] = useState(42.5)
  const [goroutinesCount, setGoroutinesCount] = useState(34)

  useEffect(() => {
    const timer = setInterval(() => {
      setOffsetCounter(c => c + Math.floor(Math.random() * 3) + 1)
      setCpuPulse(40 + Math.random() * 8)
      setGoroutinesCount(32 + Math.floor(Math.random() * 5))
    }, 1000)
    return () => clearInterval(timer)
  }, [])

  const handleScroll = (e) => {
    const el = e.target
    const fraction = el.scrollTop / (el.scrollHeight - el.clientHeight)
    setScrollFraction(Math.min(Math.max(fraction, 0), 1))
  }

  // Scroll-Aware Geodesic Globe, Service Mesh, and Cyber-Shield Math Interpolation
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    let animationId
    let angleX = 0
    let angleY = 0

    // 1. Geodesic Sphere
    const spherePoints = []
    const numPoints = 80
    for (let i = 0; i < numPoints; i++) {
      const theta = Math.acos(-1 + (2 * i) / numPoints)
      const phi = Math.sqrt(numPoints * Math.PI) * theta
      spherePoints.push({
        x: 230 * Math.sin(theta) * Math.cos(phi),
        y: 230 * Math.sin(theta) * Math.sin(phi),
        z: 230 * Math.cos(theta)
      })
    }

    // 2. Service Mesh
    const meshPoints = []
    for (let i = 0; i < 20; i++) {
      meshPoints.push({ x: -230, y: -160 + i * 16, z: -80 + Math.sin(i) * 40 })
    }
    for (let i = 0; i < 40; i++) {
      meshPoints.push({ x: 0, y: -200 + i * 10, z: Math.cos(i) * 90 })
    }
    for (let i = 0; i < 20; i++) {
      meshPoints.push({ x: 230, y: -160 + i * 16, z: -80 + Math.cos(i) * 40 })
    }

    // 3. Cyber-Shield
    const shieldPoints = []
    for (let i = 0; i < numPoints; i++) {
      const t = (i / numPoints) * Math.PI * 2
      const u = -1 + (2 * i) / numPoints
      const x = 210 * Math.sin(t) * (1 - Math.abs(u) * 0.3)
      const y = 250 * u - Math.abs(x) * 0.1
      const z = 70 * Math.cos(t) * (1 - u * u)
      shieldPoints.push({ x, y, z })
    }

    const interpolate = (p1, p2, ratio) => {
      return p1.map((pt, idx) => {
        const target = p2[idx] || pt
        return {
          x: pt.x + (target.x - pt.x) * ratio,
          y: pt.y + (target.y - pt.y) * ratio,
          z: pt.z + (target.z - pt.z) * ratio
        }
      })
    }

    const draw = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height)
      
      let currentPoints = []
      let activeColor = '#22c55e' // Globe: green
      let glowColor = 'rgba(34, 197, 94, 0.4)'

      if (scrollFraction < 0.5) {
        const ratio = scrollFraction / 0.5
        currentPoints = interpolate(spherePoints, meshPoints, ratio)
        // Transition from Green to Cyan
        activeColor = `rgb(${Math.round(34 + (6 - 34) * ratio)}, ${Math.round(197 + (182 - 197) * ratio)}, ${Math.round(94 + (212 - 94) * ratio)})`
        glowColor = `rgba(${Math.round(34 + (6 - 34) * ratio)}, ${Math.round(197 + (182 - 197) * ratio)}, ${Math.round(94 + (212 - 94) * ratio)}, 0.4)`
      } else {
        const ratio = (scrollFraction - 0.5) / 0.5
        currentPoints = interpolate(meshPoints, shieldPoints, ratio)
        // Transition from Cyan to Neon Green
        activeColor = `rgb(${Math.round(6 + (0 - 6) * ratio)}, ${Math.round(182 + (255 - 182) * ratio)}, ${Math.round(212 + (102 - 212) * ratio)})`
        glowColor = `rgba(${Math.round(6 + (0 - 6) * ratio)}, ${Math.round(182 + (255 - 182) * ratio)}, ${Math.round(212 + (102 - 212) * ratio)}, 0.4)`
      }

      angleX += 0.005
      angleY += 0.007

      const cosX = Math.cos(angleX)
      const sinX = Math.sin(angleX)
      const cosY = Math.cos(angleY)
      const sinY = Math.sin(angleY)

      const rotatedPoints = currentPoints.map(p => {
        let x1 = p.x * cosY - p.z * sinY
        let z1 = p.z * cosY + p.x * sinY
        let y2 = p.y * cosX - z1 * sinX
        let z2 = z1 * cosX + p.y * sinX
        return { x: x1, y: y2, z: z2 }
      })

      ctx.lineWidth = 0.5
      ctx.strokeStyle = activeColor
      ctx.globalAlpha = 0.25

      for (let i = 0; i < rotatedPoints.length; i++) {
        const pA = rotatedPoints[i]
        const scaleA = 300 / (300 + pA.z)
        const screenXA = canvas.width / 2 + pA.x * scaleA
        const screenYA = canvas.height / 2 + pA.y * scaleA

        for (let j = i + 1; j < rotatedPoints.length; j++) {
          const pB = rotatedPoints[j]
          const dx = pA.x - pB.x
          const dy = pA.y - pB.y
          const dz = pA.z - pB.z
          const dist = Math.sqrt(dx * dx + dy * dy + dz * dz)

          let limit = 95
          if (scrollFraction > 0.4 && scrollFraction < 0.6) {
            limit = 140
          }

          if (dist < limit) {
            const scaleB = 300 / (300 + pB.z)
            const screenXB = canvas.width / 2 + pB.x * scaleB
            const screenYB = canvas.height / 2 + pB.y * scaleB

            ctx.beginPath()
            ctx.moveTo(screenXA, screenYA)
            ctx.lineTo(screenXB, screenYB)
            ctx.stroke()
          }
        }
      }

      ctx.globalAlpha = 1.0
      rotatedPoints.forEach((p, idx) => {
        const scale = 300 / (300 + p.z)
        const screenX = canvas.width / 2 + p.x * scale
        const screenY = canvas.height / 2 + p.y * scale
        const r = Math.max(1, 2.5 * scale)

        ctx.fillStyle = activeColor
        ctx.beginPath()
        ctx.arc(screenX, screenY, r, 0, Math.PI * 2)
        ctx.fill()

        if (p.z < 0 && idx % 4 === 0) {
          ctx.fillStyle = glowColor
          ctx.beginPath()
          ctx.arc(screenX, screenY, r * 3, 0, Math.PI * 2)
          ctx.fill()
        }

        // SCI-FI NODE DIAGNOSTIC TRACKING OVERLAYS
        const nodeLabels = {
          10: "Docker Active Scanner [HEALTHY]",
          25: "AST Code Analyzer [18 nodes]",
          45: `Kafka Broker [Offset: ${314 + offsetCounter}]`,
          60: `Jarvis AI [Threat Level: ${cpuPulse.toFixed(1)}%]`,
          72: "Postgres Audit [login_log persistent]",
          5: "Redis IP Cache [12 IPs blocked]"
        }

        if (nodeLabels[idx]) {
          const text = nodeLabels[idx]
          
          // Draw larger blinking key node
          ctx.fillStyle = '#ffffff'
          ctx.beginPath()
          ctx.arc(screenX, screenY, r * 2.2, 0, Math.PI * 2)
          ctx.fill()

          ctx.strokeStyle = activeColor
          ctx.lineWidth = 1.5
          ctx.beginPath()
          ctx.arc(screenX, screenY, r * 4.5 + Math.sin(Date.now() / 150) * 2, 0, Math.PI * 2)
          ctx.stroke()

          // Offset variables to prevent overlap
          let dx = 80;
          let dy = -30;
          if (idx === 10) { dx = -150; dy = -50; }
          if (idx === 25) { dx = 150; dy = -40; }
          if (idx === 45) { dx = -160; dy = 30; }
          if (idx === 60) { dx = 160; dy = 40; }
          if (idx === 72) { dx = -150; dy = 70; }
          if (idx === 5) { dx = 150; dy = -70; }

          const lineEndX = screenX + dx
          const lineEndY = screenY + dy

          // Thin leader dotted lines
          ctx.setLineDash([2, 2])
          ctx.strokeStyle = activeColor
          ctx.lineWidth = 0.8
          ctx.globalAlpha = 0.6
          ctx.beginPath()
          ctx.moveTo(screenX, screenY)
          ctx.lineTo(lineEndX, lineEndY)
          ctx.stroke()
          ctx.setLineDash([])

          // Floating glassmorphic diagnostics label cards
          ctx.fillStyle = 'rgba(3, 7, 18, 0.85)'
          ctx.strokeStyle = activeColor
          ctx.lineWidth = 1.0
          ctx.globalAlpha = 0.9

          ctx.font = 'bold 9px monospace'
          const textWidth = ctx.measureText(text).width
          const padX = 8
          const padY = 4
          const boxW = textWidth + padX * 2
          const boxH = 18
          // Center the box horizontally depending on the side
          const boxX = dx < 0 ? lineEndX - boxW : lineEndX
          const boxY = lineEndY - 9

          // Draw label box background & border
          ctx.fillRect(boxX, boxY, boxW, boxH)
          ctx.strokeRect(boxX, boxY, boxW, boxH)

          // Glowing text rendering
          ctx.fillStyle = '#ffffff'
          ctx.fillText(text, boxX + padX, boxY + 12)
          
          ctx.globalAlpha = 1.0
          ctx.lineWidth = 0.5
        }
      })

      if (scrollFraction > 0.1) {
        ctx.globalAlpha = 0.8
        ctx.fillStyle = '#ffffff'
        for (let i = 0; i < 4; i++) {
          const p = rotatedPoints[(Math.floor(Date.now() / 200) + i * 15) % rotatedPoints.length]
          const scale = 300 / (300 + p.z)
          const screenX = canvas.width / 2 + p.x * scale
          const screenY = canvas.height / 2 + p.y * scale
          ctx.beginPath()
          ctx.arc(screenX, screenY, 2, 0, Math.PI * 2)
          ctx.fill()
        }
      }

      animationId = requestAnimationFrame(draw)
    }

    draw()
    return () => cancelAnimationFrame(animationId)
  }, [scrollFraction])

  const leftPercent = 50;
  const topPercent = 50;
  const scale = 1.7 - 0.25 * scrollFraction;

  const scrollToSec = (id) => {
    containerRef.current.querySelector(id).scrollIntoView({ behavior: 'smooth' })
  }

  return (
    <div className="landing-root" ref={containerRef} onScroll={handleScroll}>
      {/* HEADER */}
      <header className="landing-header">
        <div className="landing-logo-container">
          <BrandLogo size={32} className="landing-logo-img" />
          <span className="landing-logo-text">SecureStream AI</span>
        </div>
        <nav className="landing-nav">
          <a href="#features" className="landing-nav-link" onClick={(e) => { e.preventDefault(); scrollToSec('#features') }}>Özellikler</a>
          <a href="#modules" className="landing-nav-link" onClick={(e) => { e.preventDefault(); scrollToSec('#modules') }}>Sistem Detayları</a>
          <a href="#architecture" className="landing-nav-link" onClick={(e) => { e.preventDefault(); scrollToSec('#architecture') }}>Kurumsal Mimari</a>
          <button className="landing-btn secondary" onClick={onLogin}>Sisteme Giriş Yap</button>
        </nav>
      </header>

      {/* FIXED DYNAMIC 3D CANVAS */}
      <div 
        className="landing-3d-canvas-wrap"
        style={{
          left: `${leftPercent}%`,
          top: `${topPercent}%`,
          transform: `translate(-50%, -50%) scale(${scale})`,
          width: '1200px',
          height: '1200px',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <canvas ref={canvasRef} width="1200" height="1200" />
        <div className="landing-3d-label">
          {scrollFraction < 0.35 && 'AKTİF KORUMA: KÜRESEL GÖZLEM'}
          {scrollFraction >= 0.35 && scrollFraction < 0.7 && 'KAFKA VERİ AKIŞI: SERVİS AĞI'}
          {scrollFraction >= 0.7 && 'OTONOM ZIRH: SİBER KALKAN'}
        </div>

        {/* SCI-FI HUD TELEMETRY OVERLAYS */}
        <div className="hud-panel top-left">
          <div className="hud-header"><span className="hud-dot pulsing"></span> SYSTEM KERNEL STATE</div>
          <div className="hud-row"><span>Uptime:</span> <span>99.98%</span></div>
          <div className="hud-row"><span>Go Goroutines:</span> <span>{goroutinesCount} active</span></div>
          <div className="hud-row"><span>Docker Links:</span> <span>5 / 5 OK</span></div>
        </div>

        <div className="hud-panel top-right">
          <div className="hud-header"><span className="hud-dot pulsing"></span> KAFKA REALTIME TELEMETRY</div>
          <div className="hud-row"><span>Offset:</span> <span className="neon-value">{314 + offsetCounter}</span></div>
          <div className="hud-row"><span>Partition:</span> <span>0</span></div>
          <div className="hud-row"><span>Topic:</span> <span className="neon-value">"telemetry-data"</span></div>
        </div>

        <div className="hud-panel bottom-left">
          <div className="hud-header"><span className="hud-dot pulsing"></span> JARVIS AI CONTAINMENT</div>
          <div className="hud-row"><span>Firewall:</span> <span className="neon-value">ARMED</span></div>
          <div className="hud-row"><span>CPU Overhead:</span> <span>{cpuPulse.toFixed(1)}%</span></div>
          <div className="hud-row"><span>IP Block List:</span> <span className="threat-value">12 cached</span></div>
        </div>

        <div className="hud-panel bottom-right">
          <div className="hud-header"><span className="hud-dot pulsing"></span> AST AUTO-DISCOVERY</div>
          <div className="hud-row"><span>Scan Engine:</span> <span>Go Static</span></div>
          <div className="hud-row"><span>Graph Nodes:</span> <span className="neon-value">18 resolved</span></div>
          <div className="hud-row"><span>API Route:</span> <span>/api/discover</span></div>
        </div>
      </div>

      {/* HERO SECTION - FULL CENTERED VIEWPORT OVERLAY */}
      <section className="landing-section hero" id="hero">
        <div className="landing-hero-center-content">
          <div className="landing-badge">DİNAMİK TOPOLOJİ & SİBER KALKAN</div>
          <h1 className="landing-hero-title">
            SecureStream AI
          </h1>
          <p className="landing-hero-desc">
            Yapay Zeka Destekli Canlı Altyapı Auto-Discovery & Fiziksel Apache Kafka Log Analiz Terminali. Kurumsal kod tabanlarınızı saniyeler içinde tarayın ve otonom zırh hattınızı devreye alın.
          </p>
          <div className="landing-hero-ctas">
            <button className="landing-btn primary" onClick={onLogin}>Sisteme Giriş Yap</button>
            <button className="landing-btn secondary" onClick={() => scrollToSec('#features')}>Sistem Detaylarını Keşfet</button>
          </div>
        </div>
      </section>

      {/* SCROLLABLE LEFT COLUMN */}
      <div className="landing-scroll-content">
        {/* FEATURES SECTION */}
        <section className="landing-section" id="features">
          <h2 className="landing-section-title">Temel Yeteneklerimiz</h2>
          <p className="landing-section-desc">
            Geleneksel ve statik izleme araçlarını unutun. Canlı, otonom ve etkileşimli modern siber savunma ekosistemiyle tanışın.
          </p>

          <div className="landing-text-card">
            <h3>AST Tabanlı Canlı Ağ Topolojisi Haritası</h3>
            <p>Auto-Discovery özelliğimizi kullanarak kod tabanınızı saniyeler içinde statik olarak tarayın. Sistemleriniz, veritabanlarınız ve mikroservisleriniz arasındaki bağlantıları neon parçacık animasyonlarıyla anlık izleyin.</p>
          </div>

          <div className="landing-text-card">
            <h3>Resmi Apache Kafka Entegrasyonu</h3>
            <p>Simüle edilmiş kuyrukları geride bırakın. Docker Compose üzerinde KRaft modda çalışan gerçek Kafka Broker kuyruklarına bağlanarak Offset ve Partition bazlı fiziksel veri akışlarını izleyin.</p>
          </div>

          <div className="landing-text-card">
            <h3>Otonom JARVIS AI Kalkanı</h3>
            <p>Saldırı girişimleri (SQL Injection, Brute Force) anında tespit edilir. JARVIS AI, saldırgan IP adresini otomatik olarak bloklayarak ağ topolojisindeki kırmızı lazer kalkanlarıyla savunma hattı kurar.</p>
          </div>
        </section>

        {/* DETAILED SYSTEM MODULES */}
        <section className="landing-section" id="modules">
          <h2 className="landing-section-title">Sistem Yetenekleri ve Ayrıntıları</h2>
          <p className="landing-section-desc">
            SecureStream AI, kurumsal altyapınızın en derin noktalarına nüfuz ederek tam bir koruma kalkanı oluşturur.
          </p>

          <div className="landing-text-card">
            <h3>AST Static Code Analyzer</h3>
            <p>Go, JavaScript ve Python projelerinizin kaynak kodlarını statik olarak analiz eder. Kod içindeki veritabanı bağlantı metinlerini, HTTP istemcilerini, API rotalarını ve gRPC çağrılarını otomatik olarak yakalayarak fiziksel bağlantı haritasını çıkartır.</p>
          </div>

          <div className="landing-text-card">
            <h3>Docker Container Active Scanner</h3>
            <p>Altyapınızdaki tüm konteynerleri ve mikroservisleri canlı olarak sorgular. Çalışma durumlarını, ağ ayarlarını ve açık portları (örneğin Kafka 9092, Redis 6379, PostgreSQL 5432) eşzamanlı olarak topoloji haritanıza işler.</p>
          </div>

          <div className="landing-text-card">
            <h3>Canlı Fiziksel Kafka Offset Takibi</h3>
            <p>Saniyede binlerce log akışı işlenirken Kafka broker üzerindeki Partition ve Offset değerlerini anlık olarak okur. Hangi servisin hangi veri kanalına (Producer/Consumer) ne kadar veri bastığını canlı olarak analiz eder.</p>
          </div>

          <div className="landing-text-card">
            <h3>Otonom Tehdit Karantinası (Jarvis)</h3>
            <p>Log akışlarında şüpheli SQL Injection (örneğin SELECT * FROM, UNION SELECT) veya Brute Force şablonları yakalandığı an otonom zırh devreye girer. Saldırgan IP adresi otomatik olarak bloke edilerek topolojide izole edilir.</p>
          </div>
        </section>

        {/* ARCHITECTURE SECTION */}
        <section className="landing-section" id="architecture">
          <h2 className="landing-section-title">Güçlü ve Ölçeklenebilir Mimari</h2>
          <p className="landing-section-desc">
            Sıfır veri kaybı ve maksimum hız için tasarlanmış kurumsal altyapı bileşenlerimiz.
          </p>

          <div className="landing-text-card">
            <h3>Go Engine & Concurrency Model</h3>
            <p>Eşzamanlı (Concurrent) log işleme mimarisi tamamen Go kanalları (goroutines & channels) üzerine inşa edilmiştir. Yüksek işlem hacimli log akışları CPU kilitlenmesi yaşanmadan anlık olarak işlenip yönlendirilir.</p>
          </div>

          <div className="landing-text-card">
            <h3>PostgreSQL B2B Audit Persistence</h3>
            <p>Bütün kullanıcı giriş hareketleri, başarılı ve başarısız oturum denemeleri, audit uyumluluğu için PostgreSQL veritabanımızdaki login_log tablosuna istemci IP adresleriyle birlikte kalıcı olarak kaydedilir.</p>
          </div>

          <div className="landing-text-card">
            <h3>Redis In-Memory Firewall Cache</h3>
            <p>JARVIS AI tarafından engellenen zararlı IP adresleri ve otonom siber kalkan verileri Redis bellek içi veritabanında saklanır. Bu sayede her log kontrolünde veritabanı yorulmaz ve filtreleme sıfır gecikmeyle çalışır.</p>
          </div>
        </section>
      </div>

      {/* FOOTER */}
      <footer className="landing-footer">
        <div>© 2026 SecureStream AI. Tüm Hakları Saklıdır.</div>
      </footer>
    </div>
  )
}

function LoginPage({ onBack, onLoginSuccess }) {
  const [isRegister, setIsRegister] = useState(false)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('demo@securestream.dev')
  const [password, setPassword] = useState('demo12345')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    setSuccess('')

    if (!email.trim() || !password.trim() || (isRegister && !name.trim())) {
      setError('Lütfen tüm alanları eksiksiz doldurun.')
      setLoading(false)
      return
    }

    try {
      if (isRegister) {
        // HESAP OLUŞTURMA (REGISTER)
        const res = await fetch(`${API_URL}/register`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name, email, password })
        })
        const data = await res.json()
        if (res.ok && data.status === 'success') {
          setSuccess('Hesabınız başarıyla oluşturuldu! Giriş yapılıyor...')
          setTimeout(() => {
            onLoginSuccess(data.api_key)
          }, 1200)
        } else {
          setError(data.error || 'Hesap oluşturulurken bir hata oluştu.')
        }
      } else {
        // GİRİŞ YAPMA (LOGIN)
        const res = await fetch(`${API_URL}/login`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email, password })
        })
        const data = await res.json()
        if (res.ok && data.status === 'success') {
          onLoginSuccess(data.api_key)
        } else {
          setError(data.error || 'E-posta veya şifre hatalı.')
        }
      }
    } catch (err) {
      setError('Bağlantı hatası: Sunucuya erişilemiyor.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-root">
      <div className="login-orb" />
      <div className="login-orb blue" />

      <div className="login-card">
        <div className="login-shield-wrapper">
          <BrandLogo size={72} className="login-brand-logo-img" />
        </div>

        <h2 className="login-title">SecureStream AI</h2>
        <p className="login-subtitle">
          Altyapınızı canlı izlemek için oturum açın veya tamamen ücretsiz yeni bir hesap oluşturun.
        </p>

        {/* TABS SELECTOR */}
        <div className="login-tabs">
          <button 
            type="button" 
            className={`login-tab-btn ${!isRegister ? 'active' : ''}`}
            onClick={() => { setIsRegister(false); setError(''); setSuccess(''); }}
          >
            Giriş Yap
          </button>
          <button 
            type="button" 
            className={`login-tab-btn ${isRegister ? 'active' : ''}`}
            onClick={() => { setIsRegister(true); setError(''); setSuccess(''); }}
          >
            Hesap Oluştur
          </button>
        </div>

        <form onSubmit={handleSubmit} className="login-form">
          {isRegister && (
            <div className="login-group">
              <label className="login-label">TAM ADINIZ</label>
              <div className="login-input-wrapper">
                <input
                  type="text"
                  className="login-input"
                  placeholder="Örn: Mert Işıksal"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                />
              </div>
            </div>
          )}

          <div className="login-group">
            <label className="login-label">E-POSTA ADRESİ</label>
            <div className="login-input-wrapper">
              <input
                type="email"
                className="login-input"
                placeholder="Örn: mert@securestream.ai"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
          </div>

          <div className="login-group">
            <label className="login-label">ŞİFRE</label>
            <div className="login-input-wrapper">
              <input
                type="password"
                className="login-input"
                placeholder="••••••••"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
          </div>

          {error && <div className="login-error">{error}</div>}
          {success && <div className="login-success">{success}</div>}

          <button type="submit" className="landing-btn primary" disabled={loading}>
            {loading ? 'İşlem Doğrulanıyor...' : (isRegister ? 'Ücretsiz Hesap Oluştur' : 'Yönetim Paneline Giriş Yap')}
          </button>
        </form>

        {/* API KEY HELPER GUIDE */}
        <div className="login-helper-box">
          <div className="helper-title">BEDAVA KULLANIM & DEMO DETAYLARI</div>
          <div className="helper-section">
            <span className="helper-tag">DEMO GİRİŞ</span>
            <p>Yukarıdaki varsayılan Demo e-posta adresi ve şifre ile doğrudan <b>Yönetim Paneline</b> ulaşabilirsiniz.</p>
          </div>
          <div className="helper-section">
            <span className="helper-tag mock">AWS SAAS FREE</span>
            <p>Bu sistem AWS üzerinde tamamen bedelsiz ve açık kaynaklı olarak çalışır. Kurulum ücreti veya abonelik bedeli yoktur.</p>
          </div>
        </div>

        <button className="login-back-btn" onClick={onBack}>
          ← Ana Sayfaya Dön
        </button>
      </div>
    </div>
  )
}

/* ── NAV ICONS (inline SVGs) ── */
const icons = {
  topology: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="3"/><circle cx="4" cy="6" r="2"/><circle cx="20" cy="6" r="2"/><circle cx="4" cy="18" r="2"/><circle cx="20" cy="18" r="2"/><line x1="6" y1="7" x2="10" y2="10"/><line x1="18" y1="7" x2="14" y2="10"/><line x1="6" y1="17" x2="10" y2="14"/><line x1="18" y1="17" x2="14" y2="14"/></svg>,
  alerts: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>,
  logs: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>,
  stats: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>,
  uptime: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>,
  firewall: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>,
  discovery: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/><path d="M16 12h-4"/></svg>,
}

/* ── MAIN APP ── */
export default function App() {
  const [apiKey, setApiKey] = useState(localStorage.getItem('ss_api_key') || 'dev-api-key-12345')
  const [viewMode, setViewMode] = useState(localStorage.getItem('ss_logged_in') === 'true' ? 'dashboard' : 'landing')
  
  const [menuOpen, setMenuOpen] = useState(true)
  const [page, setPage] = useState('topology')
  const [logs, setLogs] = useState([])
  const [alerts, setAlerts] = useState([])
  const [wsState, setWsState] = useState('disconnected')
  const [toasts, setToasts] = useState([])
  const [stats, setStats] = useState({ total_logs: 0, total_alerts: 0, by_severity: {} })
  const [activeFlows, setActiveFlows] = useState([])
  const [uptimeData, setUptimeData] = useState([])
  const [sysActions, setSysActions] = useState({ blocked_ips: [] })
  const [selectedNode, setSelectedNode] = useState(null)
  const [copiedKey, setCopiedKey] = useState(false)
  const wsRef = useRef(null)

  const toastIdRef = useRef(0)
  const pushToast = useCallback((alert) => {
    const id = ++toastIdRef.current
    setToasts([{ ...alert, id }])
    setTimeout(() => setToasts(p => p.filter(t => t.id !== id)), 2600)
  }, [])

  const fetchAlerts = useCallback(async () => {
    try {
      const res = await fetch(`${API_URL}/alerts`, { headers: { 'X-API-Key': apiKey } })
      const data = await res.json()
      if (data.alerts) setAlerts(prev => {
        const dbIds = new Set(data.alerts.map(a => a.id))
        const wsOnly = prev.filter(a => !a.id || !dbIds.has(a.id))
        return [...wsOnly, ...data.alerts].slice(0, 200)
      })
    } catch (_) {}
  }, [apiKey])

  const fetchStats = useCallback(async () => {
    try { const r = await fetch(`${API_URL}/stats`, { headers: { 'X-API-Key': apiKey } }); setStats(await r.json()) } catch(_){}
  }, [apiKey])

  const fetchUptime = useCallback(async () => {
    try { const r = await fetch(`${API_URL}/uptime`, { headers: { 'X-API-Key': apiKey } }); const d = await r.json(); if(d.uptime) setUptimeData(d.uptime) } catch(_){}
  }, [apiKey])

  const fetchActions = useCallback(async () => {
    try { const r = await fetch(`${API_URL}/actions`, { headers: { 'X-API-Key': apiKey } }); setSysActions(await r.json()) } catch(_){}
  }, [apiKey])

  useEffect(() => {
    if (viewMode !== 'dashboard') return
    fetchAlerts(); fetchStats(); fetchUptime(); fetchActions()
    const iv = setInterval(() => { fetchAlerts(); fetchStats(); fetchUptime(); fetchActions() }, 10000)
    return () => clearInterval(iv)
  }, [viewMode, fetchAlerts, fetchStats, fetchUptime, fetchActions])

  useEffect(() => {
    if (viewMode !== 'dashboard') return

    const getWsUrl = () => {
      return isDev
        ? `ws://localhost:8080/api/ws?api_key=${apiKey}`
        : `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/ws?api_key=${apiKey}`
    }

    const connect = () => {
      const ws = new WebSocket(getWsUrl())
      wsRef.current = ws
      ws.onopen = () => setWsState('connected')
      ws.onclose = () => { setWsState('disconnected'); setTimeout(connect, 3000) }
      ws.onerror = () => ws.close()
      ws.onmessage = (e) => {
        const msg = JSON.parse(e.data)
        if (msg.msg_type === 'flow') {
          const now = Date.now()
          setActiveFlows(prev => [...prev.filter(f => now - f.ts < 3000), { source: msg.source, target: msg.target, ts: now }])
          return
        }
        if (msg.msg_type === 'log') {
          setLogs(prev => [...prev, { source: msg.source, raw_log: msg.raw_log, ts: new Date(msg.timestamp) }].slice(-300))
          return
        }
        const alert = msg
        alert.timestamp = new Date().toISOString()
        setAlerts(prev => [alert, ...prev].slice(0, 200))
        setLogs(prev => [...prev, { source: alert.type, raw_log: alert.message, ts: new Date() }].slice(-300))
        pushToast(alert)
        setTimeout(() => { fetchAlerts(); fetchStats() }, 1000)
      }
    }
    connect()
    return () => wsRef.current?.close()
  }, [viewMode, apiKey, pushToast, fetchAlerts, fetchStats])

  const handleLoginSuccess = (key) => {
    setApiKey(key)
    localStorage.setItem('ss_api_key', key)
    localStorage.setItem('ss_logged_in', 'true')
    setViewMode('dashboard')
  }

  const handleLogout = () => {
    localStorage.removeItem('ss_logged_in')
    setViewMode('landing')
  }

  const navTo = (p) => { setPage(p); setMenuOpen(false) }

  const critCount = stats.by_severity?.critical || 0

  const navItems = [
    { id: 'topology', label: 'Topology', icon: icons.topology, group: 'Monitoring' },
    { id: 'alerts', label: 'Alerts', icon: icons.alerts, badge: critCount, group: 'Monitoring' },
    { id: 'logs', label: 'Live Logs', icon: icons.logs, group: 'Monitoring' },
    { id: 'stats', label: 'Statistics', icon: icons.stats, group: 'Monitoring' },
    { id: 'discovery', label: 'Auto-Discovery', icon: icons.discovery, group: 'Configuration' },
    { id: 'uptime', label: 'Uptime Monitor', icon: icons.uptime, group: 'System' },
    { id: 'control', label: 'Firewall Control', icon: icons.firewall, group: 'System' },
  ]

  let lastGroup = ''

  if (viewMode === 'landing') {
    return <LandingPage onLogin={() => setViewMode('login')} />
  }

  if (viewMode === 'login') {
    return <LoginPage onBack={() => setViewMode('landing')} onLoginSuccess={handleLoginSuccess} />
  }

  return (
    <div className="app-root">
      {/* ── FULLSCREEN GRAPH ── */}
      <div className="graph-canvas">
        <NetworkGraph
          apiKey={apiKey}
          apiUrl={API_URL}
          activeFlows={activeFlows}
          alerts={alerts}
          onNodeSelect={setSelectedNode}
        />
      </div>

      {/* ── MENU TOGGLE BUTTON ── */}
      <button className="menu-toggle" id="menu-toggle-btn" onClick={() => setMenuOpen(!menuOpen)} aria-label="Toggle menu">
        <span className={`hamburger ${menuOpen ? 'open' : ''}`}>
          <span /><span /><span />
        </span>
      </button>

      {/* ── STATUS PILL ── */}
      <div className="status-pill">
        <div className={`ws-indicator ${wsState}`}>
          <span className="ws-dot" />
          {wsState === 'connected' ? 'LIVE' : 'OFFLINE'}
        </div>
      </div>

      {/* ── SLIDE-IN SIDEBAR ── */}
      <div className={`sidebar-overlay ${menuOpen ? 'visible' : ''}`} onClick={() => setMenuOpen(false)} />
      <aside className={`sidebar ${menuOpen ? 'open' : ''}`} id="sidebar-nav">
        <div className="sidebar-header">
          <div className="sidebar-brand-wrap">
            <div className="sidebar-brand-icon">
              <BrandLogo size={28} className="sidebar-brand-logo-img" />
            </div>
            <div>
              <div className="sidebar-brand">SecureStream</div>
              <div className="sidebar-sub">Security Platform</div>
            </div>
          </div>
        </div>
        <nav className="sidebar-nav">
          {navItems.map(item => {
            const showGroup = item.group !== lastGroup
            lastGroup = item.group
            return (
              <div key={item.id}>
                {showGroup && <div className="nav-group-label">{item.group}</div>}
                <button
                  className={`nav-btn ${page === item.id ? 'active' : ''}`}
                  onClick={() => navTo(item.id)}
                  id={`nav-${item.id}`}
                >
                  <span className="nav-icon">{item.icon}</span>
                  <span className="nav-label">{item.label}</span>
                </button>
              </div>
            )
          })}
        </nav>
        <div className="sidebar-footer-info">
          <div className="sidebar-api-label" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span>Oturum Anahtarı (JWT)</span>
            {copiedKey && <span style={{ color: '#10b981', fontSize: '9px', fontWeight: 'bold' }}>✓ Kopyalandı</span>}
          </div>
          <div 
            className="sidebar-api-val" 
            style={{ 
              display: 'flex', 
              alignItems: 'center', 
              justifyContent: 'space-between',
              background: 'rgba(6, 182, 212, 0.05)',
              border: '1px solid rgba(6, 182, 212, 0.15)',
              padding: '6px 10px',
              borderRadius: '6px',
              fontSize: '11px',
              color: '#a5f3fc',
              fontFamily: 'monospace',
              cursor: 'pointer',
              transition: 'all 0.2s',
              userSelect: 'none'
            }}
            onClick={() => {
              navigator.clipboard.writeText(apiKey);
              setCopiedKey(true);
              setTimeout(() => setCopiedKey(false), 2000);
            }}
            title="Kopyalamak için tıklayın"
          >
            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {apiKey && apiKey.length > 20 ? `${apiKey.substring(0, 10)}...${apiKey.substring(apiKey.length - 10)}` : apiKey}
            </span>
            <svg 
              width="13" 
              height="13" 
              viewBox="0 0 24 24" 
              fill="none" 
              stroke="currentColor" 
              strokeWidth="2" 
              strokeLinecap="round" 
              strokeLinejoin="round" 
              style={{ marginLeft: '6px', opacity: 0.7, color: copiedKey ? '#10b981' : '#06b6d4', transition: 'color 0.2s' }}
            >
              {copiedKey ? (
                <polyline points="20 6 9 17 4 12"></polyline>
              ) : (
                <>
                  <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                  <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                </>
              )}
            </svg>
          </div>
          <button 
            className="landing-btn" 
            onClick={handleLogout}
            style={{ 
              width: '100%', 
              marginTop: '12px', 
              padding: '6px 0', 
              fontSize: '11px', 
              background: 'rgba(239, 68, 68, 0.08)', 
              borderColor: 'rgba(239, 68, 68, 0.25)', 
              color: '#f87171',
              fontWeight: '600'
            }}
          >
            Sistemden Çıkış Yap
          </button>
          <div className="sidebar-version" style={{ marginTop: '12px' }}>v2.0.0 · Force Graph</div>
        </div>
      </aside>

      {/* ── OVERLAY CONTENT PANELS ── */}
      {page !== 'topology' && (
        <div className="content-overlay">
          <div className="content-overlay-header">
            <span className="content-overlay-title">
              {page === 'alerts' && 'Alert Center'}
              {page === 'logs' && 'Live Log Stream'}
              {page === 'stats' && 'Statistics & Analytics'}
              {page === 'discovery' && 'Topology Discovery'}
              {page === 'uptime' && 'Uptime Monitor'}
              {page === 'control' && 'Firewall Control'}
            </span>
            <button className="content-close-btn" onClick={() => setPage('topology')} id="close-overlay-btn">✕</button>
          </div>
          {page === 'alerts' && <AlertsPanel alerts={alerts} />}
          {page === 'logs' && <LogPanel logs={logs} />}
          {page === 'stats' && <StatsPanel stats={stats} alerts={alerts} />}
          {page === 'discovery' && <DiscoveryPanel apiKey={apiKey} />}
          {page === 'uptime' && <UptimePanel data={uptimeData} />}
          {page === 'control' && <ControlPanel actions={sysActions} />}
        </div>
      )}

      {/* ── NODE DETAIL PANEL ── */}
      {selectedNode && page === 'topology' && (
        <div className="node-detail-panel" id="node-detail-panel">
          <div className="node-detail-header">
            <div className="node-detail-color" style={{background: selectedNode.color || '#64748b'}} />
            <div className="node-detail-title-wrap">
              <div className="node-detail-name">{selectedNode.name || selectedNode.id}</div>
              {selectedNode.node_type && (
                <span className="node-detail-type">{selectedNode.node_type}</span>
              )}
            </div>
            <button className="node-detail-close" onClick={() => setSelectedNode(null)}>✕</button>
          </div>
          {selectedNode.description && (
            <div className="node-detail-desc">{selectedNode.description}</div>
          )}
          <div className="node-detail-meta">
            {selectedNode.tech && (
              <div className="node-detail-row">
                <span className="node-detail-label">Tech Stack</span>
                <span className="node-detail-value">{selectedNode.tech}</span>
              </div>
            )}
            <div className="node-detail-row">
              <span className="node-detail-label">Node ID</span>
              <span className="node-detail-value mono">{selectedNode.id}</span>
            </div>
            {selectedNode.parent && (
              <div className="node-detail-row">
                <span className="node-detail-label">Parent</span>
                <span className="node-detail-value mono">{selectedNode.parent}</span>
              </div>
            )}
            <div className="node-detail-row">
              <span className="node-detail-label">Group</span>
              <span className="node-detail-value">{selectedNode.group}</span>
            </div>
          </div>
          <div className="node-detail-section-title">Active Connections</div>
          <div className="node-detail-connections">
            {activeFlows.filter(f => f.source === selectedNode.id || f.target === selectedNode.id).length === 0 && (
              <div className="node-detail-empty">No active flows</div>
            )}
            {[...new Set(activeFlows.filter(f => f.source === selectedNode.id || f.target === selectedNode.id).map(f => f.source === selectedNode.id ? f.target : f.source))].map(peer => (
              <div className="node-detail-conn" key={peer}>
                <span className="node-conn-dot" />
                <span className="node-conn-name">{peer}</span>
                <span className="node-conn-dir">{activeFlows.some(f => f.source === selectedNode.id && f.target === peer) ? '→' : '←'}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* ── TOASTS ── */}
      <div className="toast-container">
        {toasts.map(t => (
          <div className={`toast ${t.severity}`} key={t.id}>
            <div style={{fontWeight:600,fontSize:11}}>{t.type?.replace(/_/g,' ').toUpperCase()}</div>
            <div style={{fontSize:11,color:'#94a3b8',marginTop:2}}>{t.message}</div>
          </div>
        ))}
      </div>

      <JarvisAssistant />
    </div>
  )
}