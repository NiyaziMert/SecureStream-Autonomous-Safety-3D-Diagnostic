import { useState, useEffect, useRef, useCallback } from 'react'
import './index.css'
import Topology3D from './Topology3D'
import JarvisAssistant from './JarvisAssistant'

const API_KEY = 'dev-api-key-12345'
const isDev = window.location.port === '5173';
const API_URL = isDev ? 'http://localhost:8080/api' : '/api';
const WS_URL = isDev 
  ? `ws://localhost:8080/api/ws?api_key=${API_KEY}` 
  : `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/ws?api_key=${API_KEY}`;

const SEVERITY_ORDER = { critical: 0, high: 1, medium: 2, low: 3 }
const SAMPLE_LOGS = [
  { source: 'sshd',       raw_log: 'Failed password for root from 192.168.1.105 port 22 ssh2' },
  { source: 'sshd',       raw_log: 'Failed password for invalid user admin from 10.0.0.45 port 3456' },
  { source: 'postgresql', raw_log: 'FATAL: password authentication failed for user "postgres"' },
  { source: 'sshd',       raw_log: 'Accepted password for deploy from 203.0.113.42 port 54321 ssh2' },
  { source: 'firewall',   raw_log: 'DROP IN=eth0 SRC=185.220.101.5 DST=10.0.0.1 PROTO=TCP DPT=22' },
  { source: 'sshd',       raw_log: 'sudo: user john NOT in sudoers ; TTY=pts/0' },
  { source: 'sshd',       raw_log: 'Failed password for root from 192.168.1.105 port 22 ssh2' },
  { source: 'sshd',       raw_log: 'Failed password for root from 192.168.1.105 port 22 ssh2' },
  { source: 'sshd',       raw_log: 'Failed password for root from 192.168.1.105 port 22 ssh2' },
  { source: 'sshd',       raw_log: 'Failed password for root from 192.168.1.105 port 22 ssh2' },
  { source: 'sshd',       raw_log: 'Failed password for root from 192.168.1.105 port 22 ssh2' },
]

function formatTime(d) {
  const dt = d ? new Date(d) : new Date()
  return dt.toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function SeverityBadge({ severity }) {
  return <span className={`alert-severity-badge ${severity}`}>{severity}</span>
}

function StatCard({ value, label, type }) {
  return (
    <div className={`stat-card ${type}`}>
      <div className="stat-value">{value}</div>
      <div className="stat-label">{label}</div>
    </div>
  )
}

function MiniChart({ alerts }) {
  const types = [
    { key: 'brute_force',         label: 'Brute Force',       color: '#ef4444' },
    { key: 'unauthorized_access', label: 'Yetkisiz Erişim',   color: '#f97316' },
    { key: 'db_breach_attempt',   label: 'DB İhlali',         color: '#8b5cf6' },
    { key: 'privilege_escalation',label: 'Yetki Yükseltme',   color: '#eab308' },
  ]
  const counts = types.map(t => ({
    ...t,
    count: alerts.filter(a => a.type === t.key).length,
  }))
  const max = Math.max(...counts.map(c => c.count), 1)
  return (
    <div className="mini-chart">
      {counts.map(c => (
        <div className="chart-row" key={c.key}>
          <div className="chart-label">{c.label}</div>
          <div className="chart-bar-bg">
            <div className="chart-bar-fill" style={{ width: `${(c.count / max) * 100}%`, background: c.color }} />
          </div>
          <div className="chart-count">{c.count}</div>
        </div>
      ))}
    </div>
  )
}

function LiveStream({ logs }) {
  const ref = useRef(null)
  useEffect(() => {
    if (ref.current) ref.current.scrollTop = ref.current.scrollHeight
  }, [logs])

  return (
    <div className="log-stream" ref={ref}>
      {logs.length === 0 && (
        <div className="empty-state">
          <div className="empty-text">Log bekleniyor...</div>
        </div>
      )}
      {logs.map((l, i) => (
        <div className="log-line" key={i}>
          <span className="log-time">{formatTime(l.ts)}</span>
          <span className={`log-source ${l.source}`}>{l.source}</span>
          <span className="log-text">{l.raw_log}</span>
        </div>
      ))}
    </div>
  )
}

function AlertFeed({ alerts }) {
  return (
    <div className="alert-feed">
      {alerts.length === 0 && (
        <div className="empty-state">
          <div className="empty-text">Henüz alert yok</div>
        </div>
      )}
      {alerts.map((a, i) => (
        <div className="alert-item" key={i}>
          <SeverityBadge severity={a.severity} />
          <div className="alert-content">
            <div className="alert-msg">{a.message}</div>
            <div className="alert-meta">
              {a.source_ip && <span>IP: {a.source_ip}</span>}
              {a.username  && <span>Kullanıcı: {a.username}</span>}
              <span className="alert-type-tag">{a.type}</span>
              <span className="alert-time">{formatTime(a.timestamp)}</span>
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}

function IngestPanel({ onSend }) {
  const [source, setSource] = useState('sshd')
  const [raw, setRaw]       = useState('')
  const [loading, setLoading] = useState(false)

  const submit = async () => {
    if (!raw.trim()) return
    setLoading(true)
    await onSend(source, raw)
    setRaw('')
    setLoading(false)
  }

  const loadSample = () => {
    const s = SAMPLE_LOGS[Math.floor(Math.random() * SAMPLE_LOGS.length)]
    setSource(s.source)
    setRaw(s.raw_log)
  }

  return (
    <div className="ingest-panel">
      <div className="input-group">
        <label className="input-label">Log Kaynağı</label>
        <select className="select-field" value={source} onChange={e => setSource(e.target.value)}>
          <option value="sshd">sshd (SSH)</option>
          <option value="postgresql">postgresql</option>
          <option value="firewall">firewall</option>
          <option value="custom">custom</option>
        </select>
      </div>
      <div className="input-group">
        <label className="input-label">Ham Log Satırı</label>
        <textarea
          className="textarea-field"
          value={raw}
          onChange={e => setRaw(e.target.value)}
          placeholder="Failed password for root from 192.168.1.1 port 22"
        />
      </div>
      <div style={{ display: 'flex', gap: 8 }}>
        <button className="btn btn-secondary" onClick={loadSample} style={{ flex: 1 }}>Örnek</button>
        <button className="btn btn-primary" onClick={submit} disabled={loading} style={{ flex: 2 }}>
          {loading ? 'Gönderiliyor...' : 'Gönder'}
        </button>
      </div>
    </div>
  )
}

function ToastContainer({ toasts }) {
  return (
    <div className="toast-container">
      {toasts.map(t => (
        <div className={`toast ${t.severity}`} key={t.id}>
          <div>
            <div style={{ fontWeight: 600, fontSize: 12 }}>{t.type?.replace(/_/g,' ').toUpperCase()}</div>
            <div style={{ color: 'var(--text-secondary)', fontSize: 12, marginTop: 2 }}>{t.message}</div>
          </div>
        </div>
      ))}
    </div>
  )
}

// ── MAIN APP ──────────────────────────────────────────────
export default function App() {
  const [page,        setPage]       = useState('dashboard')
  const [logs,        setLogs]       = useState([])
  const [alerts,      setAlerts]     = useState([])
  const [wsState,     setWsState]    = useState('disconnected')
  const [toasts,      setToasts]     = useState([])
  const [tenant,      setTenant]     = useState({ name: 'Demo Şirketi A.Ş.', plan: 'Pro Plan' })
  const [stats,       setStats]      = useState({ total_logs: 0, total_alerts: 0, by_severity: {}, rate_limit: null })
  const [activeFlows, setActiveFlows] = useState([])  // { source, target, ts }
  const [uptimeData,  setUptimeData]  = useState([])
  const [sysActions,  setSysActions]  = useState({ blocked_ips: [] })
  const wsRef = useRef(null)

  const pushToast = useCallback((alert) => {
    const id = Date.now()
    setToasts(p => [...p.slice(-3), { ...alert, id }])
    setTimeout(() => setToasts(p => p.filter(t => t.id !== id)), 4000)
  }, [])

  // İstatistikleri çek
  const fetchStats = useCallback(async () => {
    try {
      const res = await fetch(`${API_URL}/stats`, {
        headers: { 'X-API-Key': API_KEY },
      })
      const data = await res.json()
      setStats(data)
    } catch (_) {}
  }, [])

  // DB'den geçmiş alertleri yükle
  const fetchAlerts = useCallback(async () => {
    try {
      const res = await fetch(`${API_URL}/alerts`, {
        headers: { 'X-API-Key': API_KEY },
      })
      const data = await res.json()
      if (data.alerts) {
        setAlerts(prev => {
          const dbIds = new Set(data.alerts.map(a => a.id))
          const wsOnly = prev.filter(a => !a.id || !dbIds.has(a.id))
          return [...wsOnly, ...data.alerts].slice(0, 200)
        })
      }
    } catch (_) { /* backend çevrimdışı */ }
  }, [])

  // CSV Export
  const handleExport = () => {
    window.open(`${API_URL}/alerts/export?api_key=${API_KEY}`, '_blank');
  }

  // Uptime verilerini çek
  const fetchUptime = useCallback(async () => {
    try {
      const res = await fetch(`${API_URL}/uptime`, { headers: { 'X-API-Key': API_KEY } })
      const data = await res.json()
      if (data.uptime) setUptimeData(data.uptime)
    } catch (_) {}
  }, [])

  // Aksiyon (Block vs) verilerini çek
  const fetchActions = useCallback(async () => {
    try {
      const res = await fetch(`${API_URL}/actions`, { headers: { 'X-API-Key': API_KEY } })
      const data = await res.json()
      setSysActions(data)
    } catch (_) {}
  }, [])

  // Startup'ta veri yükleme
  useEffect(() => {
    fetch(`${API_URL}/me`, { headers: { 'X-API-Key': API_KEY } })
      .then(r => r.json())
      .then(d => setTenant({ name: d.name, plan: d.plan }))
      .catch(() => {})
    
    fetchAlerts()
    fetchStats()
    fetchUptime()
    fetchActions()

    const interval = setInterval(() => {
      fetchAlerts()
      fetchStats()
      fetchUptime()
      fetchActions()
    }, 10000) // Uptime için daha sık (10sn) yenile
    return () => clearInterval(interval)
  }, [fetchAlerts, fetchStats, fetchUptime])

  // WebSocket bağlantısı
  useEffect(() => {
    const connect = () => {
      const ws = new WebSocket(WS_URL)
      wsRef.current = ws

      ws.onopen  = () => setWsState('connected')
      ws.onclose = () => { setWsState('disconnected'); setTimeout(connect, 3000) }
      ws.onerror = () => ws.close()

      ws.onmessage = (e) => {
        const msg = JSON.parse(e.data)

        // ── Flow olayı (topoloji impulse) ──
        if (msg.msg_type === 'flow') {
          const now = Date.now()
          setActiveFlows(prev => [
            // 3 sn geçmiş akışları temizle ve yeniyi ekle
            ...prev.filter(f => now - f.ts < 3000),
            { source: msg.source, target: msg.target, ts: now },
          ])
          return
        }

        // ── Alert olayı ──
        const alert = msg
        alert.timestamp = new Date().toISOString()
        setAlerts(prev => [alert, ...prev].slice(0, 200))
        pushToast(alert)
        setTimeout(() => {
          fetchAlerts()
          fetchStats()
        }, 1000)
      }
    }
    connect()
    return () => wsRef.current?.close()
  }, [pushToast, fetchAlerts, fetchStats])

  // Log gönder
  const sendLog = useCallback(async (source, raw_log) => {
    const entry = { source, raw_log }
    setLogs(prev => [...prev, { ...entry, ts: new Date() }].slice(-300))
    try {
      await fetch(`${API_URL}/ingest`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-API-Key': API_KEY },
        body: JSON.stringify(entry),
      })
      setTimeout(() => {
        fetchAlerts()
        fetchStats()
      }, 2000)
    } catch (_) {}
  }, [fetchAlerts, fetchStats])

  // Sayaçlar (Statlar API'den geliyor)
  const criticalCount = stats.by_severity?.critical || 0
  const highCount     = stats.by_severity?.high || 0
  const mediumCount   = stats.by_severity?.medium || 0
  const totalLogsCount = stats.total_logs || logs.length

  return (
    <div className="layout">
      {/* ── SIDEBAR ── */}
      <aside className="sidebar">
        <div className="sidebar-logo">
          <div className="product-name">SecureStream</div>
          <div className="product-tag">Security Log Platform</div>
        </div>

        <nav className="sidebar-nav">
          <div className="nav-section-label">Monitöring</div>
          <button className={`nav-item ${page==='dashboard'?'active':''}`} onClick={() => setPage('dashboard')}>
            Dashboard
          </button>
          <button className={`nav-item ${page==='alerts'?'active':''}`} onClick={() => setPage('alerts')}>
            Alerts
            {criticalCount > 0 && <span className="nav-badge">{criticalCount}</span>}
          </button>
          <button className={`nav-item ${page==='ingest'?'active':''}`} onClick={() => setPage('ingest')}>
            Log Gönder
          </button>

          <div className="nav-section-label">Uyumluluk & Sistem</div>
          <button className={`nav-item ${page==='uptime'?'active':''}`} onClick={() => setPage('uptime')}>
            Uptime Monitörü
          </button>
          <button className={`nav-item ${page==='control'?'active':''}`} onClick={() => setPage('control')}>
            Sistem Denetimi
          </button>
          <button className={`nav-item ${page==='audit'?'active':''}`} onClick={() => setPage('audit')}>
            Audit Trail
          </button>
        </nav>

        <div className="sidebar-footer">
          <div className="api-key-box">
            <div className="api-key-label">API Limit</div>
            <div className="api-key-value" style={{ fontSize: 10, marginBottom: 5 }}>
              {stats.rate_limit ? `${stats.rate_limit.used} / ${stats.rate_limit.limit} req/min` : 'Loading...'}
            </div>
            <div className="api-key-label">API Key</div>
            <div className="api-key-value">dev-api-key-12345</div>
          </div>
        </div>
      </aside>

      {/* ── MAIN ── */}
      <main className="main">
        <header className="topbar">
          <div>
            <div className="topbar-title">
              {page === 'dashboard' && 'Dashboard'}
              {page === 'alerts'    && 'Alert Merkezi'}
              {page === 'ingest'    && 'Log Gönder'}
              {page === 'audit'     && 'Audit Trail'}
            </div>
            <div className="topbar-subtitle">{tenant.name} · {tenant.plan === 'pro' ? 'Pro Plan' : tenant.plan}</div>
          </div>
          <div className="topbar-actions">
            <div className={`status-pill ${wsState}`}>
              <span className="status-dot" />
              {wsState === 'connected' ? 'Canlı' : 'Bağlanıyor...'}
            </div>
          </div>
        </header>

        <div className="page">

          {/* ── DASHBOARD PAGE ── */}
          {page === 'dashboard' && (
            <>
              <div className="card" style={{ marginBottom: 16, height: 520, padding: 0, overflow: 'hidden' }}>
                <Topology3D apiKey={API_KEY} apiUrl={API_URL} activeFlows={activeFlows} />
              </div>

              <div className="stats-grid">
                <StatCard value={criticalCount} label="Kritik Alert"  type="critical" />
                <StatCard value={highCount}     label="Yüksek Alert"  type="high" />
                <StatCard value={mediumCount}   label="Orta Alert"    type="medium" />
                <StatCard value={totalLogsCount}   label="Toplam Log"   type="ok" />
              </div>

              <div className="three-col" style={{ marginBottom: 16 }}>
                <div className="card">
                  <div className="card-header">
                    <span className="card-title">Canlı Log Akışı</span>
                    <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>{logs.length} satır</span>
                  </div>
                  <div className="card-body"><LiveStream logs={logs} /></div>
                </div>

                <div className="card">
                  <div className="card-header">
                    <span className="card-title">Son Alertler</span>
                    <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>{alerts.length} toplam</span>
                  </div>
                  <div className="card-body"><AlertFeed alerts={alerts.slice(0, 20)} /></div>
                </div>
              </div>

              <div className="two-col">
                <div className="card">
                  <div className="card-header"><span className="card-title">Alert Dağılımı</span></div>
                  <div className="card-body"><MiniChart alerts={alerts} /></div>
                </div>
                <div className="card">
                  <div className="card-header"><span className="card-title">Hızlı Log Gönder</span></div>
                  <div className="card-body"><IngestPanel onSend={sendLog} /></div>
                </div>
              </div>
            </>
          )}

          {/* ── ALERTS PAGE ── */}
          {page === 'alerts' && (
            <div className="card">
              <div className="card-header">
                <span className="card-title">Tüm Alertler</span>
                <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>{alerts.length} kayıt</span>
              </div>
              {alerts.length === 0 ? (
                <div className="empty-state" style={{ height: 300 }}>
                  <div className="empty-text">Henüz alert yok — log göndererek test et</div>
                </div>
              ) : (
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Zaman</th>
                      <th>Seviye</th>
                      <th>Tür</th>
                      <th>Mesaj</th>
                      <th>IP</th>
                      <th>Kullanıcı</th>
                    </tr>
                  </thead>
                  <tbody>
                    {[...alerts].sort((a,b)=>(SEVERITY_ORDER[a.severity]??9)-(SEVERITY_ORDER[b.severity]??9)).map((a, i) => (
                      <tr key={i}>
                        <td style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{formatTime(a.timestamp)}</td>
                        <td><SeverityBadge severity={a.severity} /></td>
                        <td><span className="alert-type-tag">{a.type}</span></td>
                        <td>{a.message}</td>
                        <td style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{a.source_ip || '—'}</td>
                        <td>{a.username || '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}

          {/* ── INGEST PAGE ── */}
          {page === 'ingest' && (
            <div style={{ maxWidth: 600 }}>
              <div className="card">
                <div className="card-header"><span className="card-title">Manuel Log Gönder</span></div>
                <IngestPanel onSend={sendLog} />
              </div>
              <div className="card" style={{ marginTop: 16 }}>
                <div className="card-header">
                  <span className="card-title">Örnek Loglar</span>
                  <button
                    className="btn btn-primary"
                    style={{ fontSize: 11, padding: '4px 14px' }}
                    onClick={async () => {
                      for (const s of SAMPLE_LOGS) {
                        await sendLog(s.source, s.raw_log)
                        await new Promise(r => setTimeout(r, 200))
                      }
                    }}
                  >
                    Tümünü Gönder (Brute-Force Simülasyonu)
                  </button>
                </div>
                <div style={{ padding: '12px 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {SAMPLE_LOGS.slice(0, 6).map((s, i) => (
                    <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                      <span className={`log-source ${s.source}`} style={{ padding:'2px 8px', borderRadius:4, fontSize:10, fontWeight:700, flexShrink:0, fontFamily:'var(--font-mono)' }}>{s.source}</span>
                      <span style={{ fontFamily:'var(--font-mono)', fontSize:11, color:'var(--text-secondary)', flex:1 }}>{s.raw_log}</span>
                      <button className="btn btn-secondary" style={{ fontSize:11, padding:'4px 10px' }} onClick={() => sendLog(s.source, s.raw_log)}>Gönder</button>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}

          {/* ── AUDIT TRAIL PAGE ── */}
          {page === 'audit' && (
            <div className="card">
              <div className="card-header">
                <span className="card-title">Audit Trail — KVKK/GDPR Denetim İzi</span>
                <button className="btn btn-secondary" style={{ fontSize: 12 }} onClick={handleExport}>CSV İndir</button>
              </div>
              {alerts.length === 0 ? (
                <div className="empty-state" style={{ height: 300 }}>
                  <div className="empty-text">Denetim kaydı yok</div>
                </div>
              ) : (
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>#</th><th>Zaman</th><th>Olay Türü</th><th>Seviye</th><th>Kaynak IP</th><th>Kullanıcı</th><th>Detay</th>
                    </tr>
                  </thead>
                  <tbody>
                    {alerts.map((a, i) => (
                      <tr key={i}>
                        <td style={{ color:'var(--text-muted)', fontFamily:'var(--font-mono)', fontSize:11 }}>{alerts.length - i}</td>
                        <td style={{ fontFamily:'var(--font-mono)', fontSize:11 }}>{formatTime(a.timestamp)}</td>
                        <td><span className="alert-type-tag">{a.type}</span></td>
                        <td><SeverityBadge severity={a.severity} /></td>
                        <td style={{ fontFamily:'var(--font-mono)', fontSize:11 }}>{a.source_ip || '—'}</td>
                        <td>{a.username || '—'}</td>
                        <td style={{ maxWidth: 300, overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap' }}>{a.message}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}

          {/* ── UPTIME PAGE ── */}
          {page === 'uptime' && (
            <div className="card">
              <div className="card-header">
                <span className="card-title">Servis Durumları (Uptime)</span>
              </div>
              {uptimeData.length === 0 ? (
                <div className="empty-state" style={{ height: 300 }}>
                  <div className="empty-text">Uptime verisi bekleniyor...</div>
                </div>
              ) : (
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Servis Adı</th>
                      <th>Durum</th>
                      <th>Gecikme (ms)</th>
                      <th>Uptime (%)</th>
                    </tr>
                  </thead>
                  <tbody>
                    {uptimeData.map((u, i) => (
                      <tr key={i}>
                        <td style={{ fontWeight: 600 }}>{u.service}</td>
                        <td>
                          <span className={`status-pill ${u.status === 'up' ? 'connected' : 'disconnected'}`} style={{ display: 'inline-flex', background: u.status==='up'?'rgba(34,197,94,0.1)':u.status==='degraded'?'rgba(245,158,11,0.1)':'rgba(239,68,68,0.1)', color: u.status==='up'?'#4ade80':u.status==='degraded'?'#fbbf24':'#f87171' }}>
                            <span className="status-dot" style={{ background: u.status==='up'?'#4ade80':u.status==='degraded'?'#fbbf24':'#f87171' }} />
                            {u.status.toUpperCase()}
                          </span>
                        </td>
                        <td style={{ fontFamily: 'var(--font-mono)' }}>{u.latency} ms</td>
                        <td style={{ fontFamily: 'var(--font-mono)' }}>{u.uptime_percent?.toFixed(2)}%</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}

          {/* ── SYSTEM CONTROL PAGE ── */}
          {page === 'control' && (
            <div className="card">
              <div className="card-header">
                <span className="card-title">J.A.R.V.I.S Action Control</span>
              </div>
              <div style={{ marginBottom: 20 }}>
                <p style={{ color: '#94a3b8', fontSize: 13, marginBottom: 12 }}>You can monitor IPs blocked by J.A.R.V.I.S or security rules and view taken actions through this panel.</p>
                <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
                  <div className="stat-card" style={{ flex: 1, minWidth: 200, padding: 16 }}>
                    <div className="stat-title">Blocked IP Count</div>
                    <div className="stat-value" style={{ color: '#ef4444' }}>{sysActions.blocked_ips?.length || 0}</div>
                  </div>
                </div>
              </div>

              {(!sysActions.blocked_ips || sysActions.blocked_ips.length === 0) ? (
                <div className="empty-state" style={{ height: 200 }}>
                  <div className="empty-text">No blocked sources yet</div>
                </div>
              ) : (
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>IP Address</th>
                      <th>Status</th>
                      <th>Source</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sysActions.blocked_ips.map((ip, i) => (
                      <tr key={i}>
                        <td style={{ fontFamily: 'var(--font-mono)', fontWeight: 600, color: '#f87171' }}>{ip}</td>
                        <td><span className="status-pill disconnected" style={{ display: 'inline-flex', background: 'rgba(239,68,68,0.1)', color: '#f87171' }}><span className="status-dot" style={{ background: '#f87171' }} />BLOCKED</span></td>
                        <td>J.A.R.V.I.S AI</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}

        </div>
      </main>

      <ToastContainer toasts={toasts} />
      <JarvisAssistant />
    </div>
  )
}