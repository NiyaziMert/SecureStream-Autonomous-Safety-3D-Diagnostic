import { useState, useRef, useEffect, useCallback } from 'react'

const isDev  = window.location.port === '5173'
const API_URL = isDev ? 'http://localhost:8080/api' : '/api'
const API_KEY = 'dev-api-key-12345'

const BOOT_MSG = {
  role: 'jarvis',
  id: 0,
  content: 'Online and ready. Connected to the SecureStream defense grid. How may I assist you?',
}

const QUICK_CMDS = ['System status', 'Recent alerts', 'Topology status', 'Help']

/* ─── Waveform ─────────────────────────────────────────── */
function Waveform({ active }) {
  const heights = [8, 18, 28, 18, 28, 18, 8]
  return (
    <div style={{ display: 'flex', gap: 3, alignItems: 'center', height: 32 }}>
      {heights.map((h, i) => (
        <div key={i} style={{
          width: 3, borderRadius: 2,
          background: 'linear-gradient(180deg,#22d3ee,#6366f1)',
          height: active ? `${Math.random() * 22 + 6}px` : `${h}px`,
          transition: 'height 0.12s ease',
          animation: active ? `wave-bar ${0.35 + i * 0.08}s ease-in-out infinite alternate` : 'none',
        }} />
      ))}
    </div>
  )
}

/* ─── Typing dots ──────────────────────────────────────── */
function TypingDots() {
  return (
    <div style={{ display: 'flex', gap: 5, padding: '10px 14px', background: 'rgba(255,255,255,0.05)', borderRadius: '16px 16px 16px 4px', border: '1px solid rgba(99,102,241,0.2)' }}>
      {[0, 1, 2].map(i => (
        <div key={i} style={{
          width: 6, height: 6, borderRadius: '50%', background: '#818cf8',
          animation: `dot-jump 0.9s ${i * 0.15}s ease-in-out infinite`,
        }} />
      ))}
    </div>
  )
}

/* ─── Main Component ───────────────────────────────────── */
export default function JarvisAssistant() {
  const [open,      setOpen]      = useState(false)
  const [messages,  setMessages]  = useState([BOOT_MSG])
  const [input,     setInput]     = useState('')
  const [loading,   setLoading]   = useState(false)
  const [speaking,  setSpeaking]  = useState(false)
  const [listening, setListening] = useState(false)
  const [msgId,     setMsgId]     = useState(1)

  const endRef    = useRef(null)
  const inputRef  = useRef(null)
  const synthRef  = useRef(window.speechSynthesis)
  const recogRef  = useRef(null)

  /* auto-scroll */
  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [messages, loading])

  /* focus on open */
  useEffect(() => { if (open) setTimeout(() => inputRef.current?.focus(), 150) }, [open])

  /* Speech Recognition setup */
  useEffect(() => {
    const SR = window.SpeechRecognition || window.webkitSpeechRecognition
    if (!SR) return
    const r = new SR()
    r.lang = 'en-US'
    r.continuous = false
    r.interimResults = false
    r.onresult  = e => { setInput(e.results[0][0].transcript); setListening(false) }
    r.onerror   = () => setListening(false)
    r.onend     = () => setListening(false)
    recogRef.current = r
    return () => r.abort()
  }, [])

  /* Text-to-Speech */
  const speak = useCallback(text => {
    if (!synthRef.current) return
    synthRef.current.cancel()
    const u = new SpeechSynthesisUtterance(text.replace(/•/g, '').replace(/\n/g, '. '))
    const voices = synthRef.current.getVoices()
    // Find an English Male voice specifically
    u.voice = voices.find(v => v.lang.startsWith('en') && /(male|david|james|daniel|mark|guy)/i.test(v.name))
           || voices.find(v => v.lang.startsWith('en-GB')) // British english often sounds good for Jarvis
           || voices.find(v => v.lang.startsWith('en'))
           || voices[0]
    u.rate = 0.95; u.pitch = 0.85
    u.onstart = () => setSpeaking(true)
    u.onend   = () => setSpeaking(false)
    u.onerror = () => setSpeaking(false)
    synthRef.current.speak(u)
  }, [])

  /* Send message */
  const send = useCallback(async text => {
    const msg = text.trim()
    if (!msg || loading) return
    const uid = msgId; setMsgId(p => p + 1)
    setMessages(prev => [...prev, { role: 'user', id: uid, content: msg }])
    setInput('')
    setLoading(true)
    try {
      const res  = await fetch(`${API_URL}/jarvis/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-API-Key': API_KEY },
        body: JSON.stringify({ message: msg }),
      })
      const data = await res.json()
      const jid  = msgId + 1; setMsgId(p => p + 1)
      setMessages(prev => [...prev, { role: 'jarvis', id: jid, content: data.reply, action: data.action }])
      speak(data.reply)
    } catch {
      setMessages(prev => [...prev, { role: 'jarvis', id: -1, content: 'Connection error. Backend is unreachable.', action: 'error' }])
    } finally {
      setLoading(false)
    }
  }, [loading, msgId, speak])

  const toggleListen = () => {
    if (!recogRef.current) return
    if (listening) { recogRef.current.stop(); setListening(false) }
    else           { recogRef.current.start(); setListening(true) }
  }

  const actionColor = { status: '#22c55e', block_ip: '#ef4444', show_alerts: '#f97316', topology: '#6366f1', error: '#ef4444' }

  return (
    <>
      {/* ── CSS Animations ────────────────────────────── */}
      <style>{`
        @keyframes wave-bar   { to { height: 32px; } }
        @keyframes dot-jump   { 0%,100%{transform:translateY(0)} 50%{transform:translateY(-6px)} }
        @keyframes jarvis-glow{ 0%,100%{box-shadow:0 0 24px rgba(14,165,233,.5),0 0 48px rgba(99,102,241,.25)} 50%{box-shadow:0 0 36px rgba(14,165,233,.8),0 0 70px rgba(99,102,241,.45)} }
        @keyframes jarvis-pulse{ 0%,100%{transform:scale(1)} 50%{transform:scale(1.08)} }
        @keyframes slide-up   { from{opacity:0;transform:translateY(20px)} to{opacity:1;transform:translateY(0)} }
        @keyframes mic-ring   { 0%,100%{box-shadow:0 0 0 0 rgba(239,68,68,.7)} 70%{box-shadow:0 0 0 8px rgba(239,68,68,0)} }
      `}</style>

      {/* ── Floating Button ───────────────────────────── */}
      <button
        id="jarvis-toggle-btn"
        onClick={() => setOpen(o => !o)}
        title="J.A.R.V.I.S — AI Assistant"
        style={{
          position: 'fixed', bottom: 28, right: 28, zIndex: 9999,
          width: 60, height: 60, borderRadius: '50%', border: 'none',
          background: 'linear-gradient(135deg,#0ea5e9,#6366f1)',
          cursor: 'pointer',
          animation: speaking ? 'jarvis-pulse .7s ease-in-out infinite' : 'jarvis-glow 3s ease-in-out infinite',
          transition: 'transform .2s',
          transform: open ? 'scale(.9) rotate(45deg)' : 'scale(1)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}
      >
        <span style={{ fontSize: 24, lineHeight: 1, filter: 'drop-shadow(0 0 6px #fff)' }}>
          {open ? '✕' : '⚡'}
        </span>
      </button>

      {/* ── Chat Panel ────────────────────────────────── */}
      {open && (
        <div
          id="jarvis-panel"
          style={{
            position: 'fixed', bottom: 104, right: 28, zIndex: 9998,
            width: 390, height: 570,
            background: 'rgba(2,6,23,0.94)',
            backdropFilter: 'blur(28px)',
            border: '1px solid rgba(99,102,241,0.45)',
            borderRadius: 22,
            boxShadow: '0 0 80px rgba(14,165,233,.2), 0 40px 100px rgba(0,0,0,.85)',
            display: 'flex', flexDirection: 'column', overflow: 'hidden',
            animation: 'slide-up .3s ease-out',
          }}
        >
          {/* Header */}
          <div style={{
            padding: '14px 18px',
            background: 'linear-gradient(135deg,rgba(14,165,233,.12),rgba(99,102,241,.12))',
            borderBottom: '1px solid rgba(99,102,241,.3)',
            display: 'flex', alignItems: 'center', gap: 12,
          }}>
            <Waveform active={speaking} />
            <div>
              <div style={{ fontWeight: 800, fontSize: 13, letterSpacing: 4, color: '#e2e8f0' }}>J.A.R.V.I.S</div>
              <div style={{ fontSize: 10, color: '#64748b', letterSpacing: 1.5, marginTop: 1 }}>
                {speaking ? '🔊 Speaking...' : listening ? '🎙 Listening...' : 'SecureStream AI · Active'}
              </div>
            </div>
            <button
              onClick={() => { synthRef.current?.cancel(); setOpen(false) }}
              style={{ marginLeft: 'auto', background: 'none', border: 'none', color: '#475569', cursor: 'pointer', fontSize: 20, lineHeight: 1, padding: 4 }}
            >×</button>
          </div>

          {/* Messages */}
          <div style={{ flex: 1, overflowY: 'auto', padding: 14, display: 'flex', flexDirection: 'column', gap: 10 }}>
            {messages.map(msg => (
              <div key={msg.id} style={{ display: 'flex', justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start', gap: 8, alignItems: 'flex-end' }}>
                {msg.role === 'jarvis' && (
                  <div style={{ width: 28, height: 28, borderRadius: '50%', background: 'linear-gradient(135deg,#0ea5e9,#6366f1)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 13, flexShrink: 0 }}>⚡</div>
                )}
                <div style={{
                  maxWidth: '80%', padding: '10px 14px',
                  borderRadius: msg.role === 'user' ? '16px 16px 4px 16px' : '4px 16px 16px 16px',
                  background: msg.role === 'user'
                    ? 'linear-gradient(135deg,#0ea5e9,#6366f1)'
                    : 'rgba(255,255,255,0.055)',
                  border: msg.role === 'jarvis' ? `1px solid ${actionColor[msg.action] || 'rgba(99,102,241,.25)'}40` : 'none',
                  color: '#e2e8f0', fontSize: 12.5, lineHeight: 1.65,
                  whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                }}>
                  {msg.action && msg.role === 'jarvis' && (
                    <div style={{ fontSize: 10, color: actionColor[msg.action] || '#6366f1', marginBottom: 5, fontWeight: 700, letterSpacing: 1 }}>
                      {({ block_ip: '🚫 IP BLOCKED', show_alerts: '⚠️ SECURITY ALERT', status: '📡 SYSTEM STATUS', topology: '🌐 TOPOLOGY', error: '❌ ERROR' })[msg.action] || msg.action.toUpperCase()}
                    </div>
                  )}
                  {msg.content}
                </div>
                {msg.role === 'user' && (
                  <div style={{ width: 28, height: 28, borderRadius: '50%', background: 'rgba(255,255,255,.08)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 13, flexShrink: 0 }}>👤</div>
                )}
              </div>
            ))}
            {loading && (
              <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
                <div style={{ width: 28, height: 28, borderRadius: '50%', background: 'linear-gradient(135deg,#0ea5e9,#6366f1)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 13 }}>⚡</div>
                <TypingDots />
              </div>
            )}
            <div ref={endRef} />
          </div>

          {/* Quick Commands */}
          <div style={{ padding: '6px 14px', display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            {QUICK_CMDS.map(cmd => (
              <button
                key={cmd}
                onClick={() => send(cmd)}
                style={{
                  fontSize: 10.5, padding: '4px 10px', borderRadius: 20, cursor: 'pointer',
                  background: 'rgba(99,102,241,.12)', border: '1px solid rgba(99,102,241,.3)',
                  color: '#a5b4fc', transition: 'all .2s', whiteSpace: 'nowrap',
                }}
                onMouseEnter={e => { e.target.style.background = 'rgba(99,102,241,.25)' }}
                onMouseLeave={e => { e.target.style.background = 'rgba(99,102,241,.12)' }}
              >{cmd}</button>
            ))}
          </div>

          {/* Input Area */}
          <div style={{
            padding: '10px 14px 14px',
            borderTop: '1px solid rgba(99,102,241,.2)',
            display: 'flex', gap: 8, alignItems: 'center',
          }}>
            {/* Mic Button */}
            <button
              id="jarvis-mic-btn"
              onClick={toggleListen}
              title="Voice command"
              style={{
                width: 38, height: 38, borderRadius: '50%', border: 'none',
                background: listening ? 'rgba(239,68,68,.2)' : 'rgba(255,255,255,.06)',
                color: listening ? '#ef4444' : '#64748b',
                cursor: recogRef.current ? 'pointer' : 'not-allowed',
                fontSize: 16, flexShrink: 0, transition: 'all .2s',
                animation: listening ? 'mic-ring 1.2s ease-in-out infinite' : 'none',
              }}
            >🎙</button>

            {/* Text Input */}
            <input
              ref={inputRef}
              id="jarvis-input"
              value={input}
              onChange={e => setInput(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && !e.shiftKey && send(input)}
              placeholder="Enter command or ask a question..."
              style={{
                flex: 1, background: 'rgba(255,255,255,.06)',
                border: '1px solid rgba(99,102,241,.3)',
                borderRadius: 12, padding: '9px 13px',
                color: '#e2e8f0', fontSize: 13, outline: 'none',
                transition: 'border-color .2s',
              }}
              onFocus={e => { e.target.style.borderColor = 'rgba(99,102,241,.7)' }}
              onBlur={e => { e.target.style.borderColor = 'rgba(99,102,241,.3)' }}
            />

            {/* Send Button */}
            <button
              id="jarvis-send-btn"
              onClick={() => send(input)}
              disabled={!input.trim() || loading}
              style={{
                width: 38, height: 38, borderRadius: '50%', border: 'none',
                background: input.trim() && !loading ? 'linear-gradient(135deg,#0ea5e9,#6366f1)' : 'rgba(255,255,255,.06)',
                color: input.trim() && !loading ? '#fff' : '#334155',
                cursor: input.trim() && !loading ? 'pointer' : 'default',
                fontSize: 16, flexShrink: 0, transition: 'all .2s',
              }}
            >➤</button>
          </div>
        </div>
      )}
    </>
  )
}
