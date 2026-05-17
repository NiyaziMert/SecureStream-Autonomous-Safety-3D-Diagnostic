import { useState, useEffect, useRef, useCallback } from 'react'
import './index.css'
import NetworkGraph from './NetworkGraph'
import JarvisAssistant from './JarvisAssistant'

const API_KEY = 'dev-api-key-12345'
const isDev = window.location.port === '5173'
const API_URL = isDev ? 'http://localhost:8080/api' : '/api'
const WS_URL = isDev
  ? `ws://localhost:8080/api/ws?api_key=${API_KEY}`
  : `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/ws?api_key=${API_KEY}`

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

/* ── NAV ICONS (inline SVGs) ── */
const icons = {
  topology: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="3"/><circle cx="4" cy="6" r="2"/><circle cx="20" cy="6" r="2"/><circle cx="4" cy="18" r="2"/><circle cx="20" cy="18" r="2"/><line x1="6" y1="7" x2="10" y2="10"/><line x1="18" y1="7" x2="14" y2="10"/><line x1="6" y1="17" x2="10" y2="14"/><line x1="18" y1="17" x2="14" y2="14"/></svg>,
  alerts: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>,
  logs: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>,
  stats: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>,
  uptime: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>,
  firewall: <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>,
}

/* ── MAIN APP ── */
export default function App() {
  const [menuOpen, setMenuOpen] = useState(false)
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
  const wsRef = useRef(null)

  const toastIdRef = useRef(0)
  const pushToast = useCallback((alert) => {
    const id = ++toastIdRef.current
    setToasts([{ ...alert, id }])
    setTimeout(() => setToasts(p => p.filter(t => t.id !== id)), 2600)
  }, [])

  const fetchAlerts = useCallback(async () => {
    try {
      const res = await fetch(`${API_URL}/alerts`, { headers: { 'X-API-Key': API_KEY } })
      const data = await res.json()
      if (data.alerts) setAlerts(prev => {
        const dbIds = new Set(data.alerts.map(a => a.id))
        const wsOnly = prev.filter(a => !a.id || !dbIds.has(a.id))
        return [...wsOnly, ...data.alerts].slice(0, 200)
      })
    } catch (_) {}
  }, [])

  const fetchStats = useCallback(async () => {
    try { const r = await fetch(`${API_URL}/stats`, { headers: { 'X-API-Key': API_KEY } }); setStats(await r.json()) } catch(_){}
  }, [])

  const fetchUptime = useCallback(async () => {
    try { const r = await fetch(`${API_URL}/uptime`, { headers: { 'X-API-Key': API_KEY } }); const d = await r.json(); if(d.uptime) setUptimeData(d.uptime) } catch(_){}
  }, [])

  const fetchActions = useCallback(async () => {
    try { const r = await fetch(`${API_URL}/actions`, { headers: { 'X-API-Key': API_KEY } }); setSysActions(await r.json()) } catch(_){}
  }, [])

  useEffect(() => {
    fetchAlerts(); fetchStats(); fetchUptime(); fetchActions()
    const iv = setInterval(() => { fetchAlerts(); fetchStats(); fetchUptime(); fetchActions() }, 10000)
    return () => clearInterval(iv)
  }, [fetchAlerts, fetchStats, fetchUptime, fetchActions])

  useEffect(() => {
    const connect = () => {
      const ws = new WebSocket(WS_URL)
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
  }, [pushToast, fetchAlerts, fetchStats])

  const navTo = (p) => { setPage(p); setMenuOpen(false) }

  const critCount = stats.by_severity?.critical || 0

  const navItems = [
    { id: 'topology', label: 'Topology', icon: icons.topology, group: 'Monitoring' },
    { id: 'alerts', label: 'Alerts', icon: icons.alerts, badge: critCount, group: 'Monitoring' },
    { id: 'logs', label: 'Live Logs', icon: icons.logs, group: 'Monitoring' },
    { id: 'stats', label: 'Statistics', icon: icons.stats, group: 'Monitoring' },
    { id: 'uptime', label: 'Uptime Monitor', icon: icons.uptime, group: 'System' },
    { id: 'control', label: 'Firewall Control', icon: icons.firewall, group: 'System' },
  ]

  let lastGroup = ''

  return (
    <div className="app-root">
      {/* ── FULLSCREEN GRAPH ── */}
      <div className="graph-canvas">
        <NetworkGraph
          apiKey={API_KEY}
          apiUrl={API_URL}
          activeFlows={activeFlows}
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
              <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="url(#brandGrad)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <defs><linearGradient id="brandGrad" x1="0%" y1="0%" x2="100%" y2="100%"><stop offset="0%" stopColor="#3b82f6"/><stop offset="100%" stopColor="#8b5cf6"/></linearGradient></defs>
                <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
              </svg>
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
                  {item.badge > 0 && <span className="nav-badge">{item.badge}</span>}
                </button>
              </div>
            )
          })}
        </nav>
        <div className="sidebar-footer-info">
          <div className="sidebar-api-label">API Key</div>
          <div className="sidebar-api-val">{API_KEY}</div>
          <div className="sidebar-version">v2.0.0 · Force Graph</div>
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
              {page === 'uptime' && 'Uptime Monitor'}
              {page === 'control' && 'Firewall Control'}
            </span>
            <button className="content-close-btn" onClick={() => setPage('topology')} id="close-overlay-btn">✕</button>
          </div>
          {page === 'alerts' && <AlertsPanel alerts={alerts} />}
          {page === 'logs' && <LogPanel logs={logs} />}
          {page === 'stats' && <StatsPanel stats={stats} alerts={alerts} />}
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