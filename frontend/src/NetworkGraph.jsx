import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import ForceGraph2D from 'react-force-graph-2d';

const FLOW_TTL_MS = 3000;

// Polyfill: ctx.roundRect is not available in all browsers
function drawRoundRect(ctx, x, y, w, h, r) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.lineTo(x + w - r, y);
  ctx.quadraticCurveTo(x + w, y, x + w, y + r);
  ctx.lineTo(x + w, y + h - r);
  ctx.quadraticCurveTo(x + w, y + h, x + w - r, y + h);
  ctx.lineTo(x + r, y + h);
  ctx.quadraticCurveTo(x, y + h, x, y + h - r);
  ctx.lineTo(x, y + r);
  ctx.quadraticCurveTo(x, y, x + r, y);
  ctx.closePath();
}

function getNodeColor(node) {
  return node.color || '#64748b';
}

export default function NetworkGraph({ apiKey, apiUrl, activeFlows = [], alerts = [], onNodeSelect }) {
  const [allData, setAllData] = useState({ nodes: [], links: [] });
  const [kafkaEnabled, setKafkaEnabled] = useState(false);
  const [expandedNodes, setExpandedNodes] = useState(new Set());
  const [hoveredNode, setHoveredNode] = useState(null);
  const [selectedNode, setSelectedNode] = useState(null);
  const fgRef = useRef();
  const containerRef = useRef(null);
  const [dims, setDims] = useState({ w: 800, h: 600 });

  const loadTopology = useCallback(() => {
    fetch(`${apiUrl}/topology`, { headers: { 'X-API-Key': apiKey } })
      .then(r => r.json())
      .then(d => setAllData(d))
      .catch(() => {});
  }, [apiUrl, apiKey]);

  const handleToggleKafka = async () => {
    const nextVal = !kafkaEnabled;
    try {
      const res = await fetch(`${apiUrl}/kafka/toggle`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-API-Key': apiKey },
        body: JSON.stringify({ enabled: nextVal })
      });
      const data = await res.json();
      if (data.status === 'success') {
        setKafkaEnabled(nextVal);
        setTimeout(loadTopology, 100);
      }
    } catch (e) {
      console.error(e);
    }
  };

  // Sync Kafka status on mount
  useEffect(() => {
    fetch(`${apiUrl}/kafka/status`, { headers: { 'X-API-Key': apiKey } })
      .then(r => r.json())
      .then(data => {
        if (data.status === 'success') {
          setKafkaEnabled(data.kafka_stream === 'enabled');
        }
      })
      .catch(() => {});
  }, [apiUrl, apiKey]);

  useEffect(() => {
    if (!containerRef.current) return;
    const ro = new ResizeObserver(entries => {
      for (const e of entries) setDims({ w: e.contentRect.width, h: e.contentRect.height });
    });
    ro.observe(containerRef.current);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    loadTopology();
    const iv = setInterval(loadTopology, 5000);
    return () => clearInterval(iv);
  }, [loadTopology]);

  // Force re-render for animations
  useEffect(() => {
    let id;
    const tick = () => { id = requestAnimationFrame(tick); };
    tick();
    return () => cancelAnimationFrame(id);
  }, []);

  const activeLinkSet = useMemo(() => {
    const now = Date.now();
    const s = new Set();
    activeFlows.filter(f => now - f.ts < FLOW_TTL_MS).forEach(f => {
      s.add(`${f.source}|||${f.target}`);
      s.add(`${f.target}|||${f.source}`);
    });
    return s;
  }, [activeFlows]);

  const isActive = useCallback(link => {
    const s = typeof link.source === 'object' ? link.source.id : link.source;
    const t = typeof link.target === 'object' ? link.target.id : link.target;
    return activeLinkSet.has(`${s}|||${t}`);
  }, [activeLinkSet]);

  const visibleData = useMemo(() => {
    const nodes = allData?.nodes || [];
    const links = allData?.links || [];
    const vis = nodes.filter(n => !n.parent || expandedNodes.has(n.parent));
    const ids = new Set(vis.map(n => n.id));
    const visLinks = links.filter(l => {
      const s = typeof l.source === 'object' ? l.source.id : l.source;
      const t = typeof l.target === 'object' ? l.target.id : l.target;
      return ids.has(s) && ids.has(t);
    });
    return { nodes: vis, links: visLinks };
  }, [allData, expandedNodes]);

  // Configure d3 forces for tighter, snugger spacing (FAZ 5.1 Sıkı Topoloji)
  useEffect(() => {
    if (!fgRef.current) return;
    const fg = fgRef.current;
    // Charge: Düşük itme kuvveti ile nodeları birbirine yaklaştırıyoruz (-400 -> -160)
    fg.d3Force('charge').strength(-160).distanceMax(500);
    // Link: Kısa ve güçlü bağlantılar (90 -> 50)
    fg.d3Force('link').distance(50).strength(0.4);
    // Center: Güçlü merkezleme kuvvetiyle dağılmayı önlüyoruz (0.05 -> 0.1)
    fg.d3Force('center').strength(0.1);
    fg.d3ReheatSimulation();
  }, [visibleData]);

  const handleClick = useCallback(node => {
    const nodes = allData?.nodes || [];
    const hasKids = nodes.some(n => n.parent === node.id);
    if (hasKids) {
      setExpandedNodes(prev => {
        const next = new Set(prev);
        if (next.has(node.id)) {
          const collapse = id => { next.delete(id); nodes.filter(n => n.parent === id).forEach(c => collapse(c.id)); };
          collapse(node.id);
        } else next.add(node.id);
        return next;
      });
    }
    setSelectedNode(node.id);
    onNodeSelect?.(node);
    if (fgRef.current) {
      fgRef.current.centerAt(node.x, node.y, 600);
      fgRef.current.zoom(2.5, 600);
    }
  }, [allData, onNodeSelect]);

  const activeNodeSet = useMemo(() => {
    const now = Date.now();
    const s = new Set();
    activeFlows.filter(f => now - f.ts < FLOW_TTL_MS).forEach(f => {
      s.add(f.source); s.add(f.target);
    });
    return s;
  }, [activeFlows]);

  // Grupları ve liderlerini önceden hesapla (Enclave Çizimi için)
  const groupLeaders = useMemo(() => {
    const leaders = {};
    const nodes = visibleData.nodes;
    nodes.forEach(n => {
      const g = n.group || 0;
      if (!leaders[g] || n.id < leaders[g].id) {
        leaders[g] = n;
      }
    });
    return leaders;
  }, [visibleData.nodes]);

  // Node Canvas render döngüsü
  const nodeCanvasObject = useCallback((node, ctx, globalScale) => {
    const x = node.x, y = node.y;
    if (!Number.isFinite(x) || !Number.isFinite(y)) return;

    const color = getNodeColor(node);
    const isHov = hoveredNode === node.id;
    const isSel = selectedNode === node.id;
    const isAct = activeNodeSet.has(node.id);
    const nodes = allData?.nodes || [];
    const hasKids = nodes.some(n => n.parent === node.id);
    const isExp = expandedNodes.has(node.id);

    // ── GÖRSEL BÜYÜTME (FAZ 5.1: 2.5 Kat Daha Büyük) ──
    const baseR = Math.sqrt(node.val || 4) * 4.5 + 4; 
    const r = baseR * (isHov ? 1.25 : isSel ? 1.15 : 1);

    // ── 🛡️ SOYUT ŞEKİL BİRLEŞTİRME ENKLAVE (HUD SHAPE MERGING) ──
    // Grubun lideri olan düğüm, o gruba ait tüm düğümleri kapsayan devasa koruyucu enklave hücresini çizer!
    const gID = node.group || 0;
    const leader = groupLeaders[gID];
    
    if (leader && leader.id === node.id) {
      const groupNodes = visibleData.nodes.filter(n => (n.group || 0) === gID && Number.isFinite(n.x) && Number.isFinite(n.y));
      
      if (groupNodes.length >= 2) {
        // 1. Ağırlık Merkezi (Center of Mass) Hesapla
        let sumX = 0, sumY = 0;
        groupNodes.forEach(n => { sumX += n.x; sumY += n.y; });
        const cx = sumX / groupNodes.length;
        const cy = sumY / groupNodes.length;

        // 2. Maksimum Uzaklığı (Yarıçap) Bul
        let maxDist = 0;
        groupNodes.forEach(n => {
          const dist = Math.sqrt((n.x - cx) ** 2 + (n.y - cy) ** 2);
          if (dist > maxDist) maxDist = dist;
        });
        
        // Düğümleri içine alacak genişlikte bir yarıçap + tampon payı
        const enclaveR = maxDist + baseR + 22;

        // 3. Grubun Tehdit Altında Olup Olmadığını Kontrol Et
        const hasThreat = groupNodes.some(n => 
          alerts.some(a => a.source === n.id || a.message?.includes(n.name || n.id))
        );

        // 4. Enclave Türünü ve Renklerini Ayarla
        let enclaveColor = '#06b6d4'; // Varsayılan: Cyan (Algorithmic Pipeline)
        let enclaveLabel = '[⚡ ALGORITHMIC CIRCUIT PIPELINE]';
        
        if (gID === 1) {
          enclaveColor = '#64748b'; // Gateway (Cobalt-Gray)
          enclaveLabel = '[🌐 EDGE GATEWAY SECURITY CELL]';
        } else if (gID === 3) {
          enclaveColor = '#22c55e'; // Database (Secure Green)
          enclaveLabel = '[🛡️ DATABASE SECURE ENCLAVE]';
        }

        // Eğer sızma/tehdit varsa enklave anında şekil değiştirir ve kızarır!
        if (hasThreat) {
          enclaveColor = '#ef4444'; // Alarm Crimson Red
          enclaveLabel = '[⚠️ THREAT BREACH ZONE DETECTED]';
        }

        ctx.save();
        
        // A. Holografik Arka Plan Gradient Dolgusu (Abstract Bubble)
        const bubbleGrad = ctx.createRadialGradient(cx, cy, enclaveR * 0.2, cx, cy, enclaveR);
        bubbleGrad.addColorStop(0, enclaveColor + '08');
        bubbleGrad.addColorStop(0.5, enclaveColor + '03');
        bubbleGrad.addColorStop(1, enclaveColor + '00');
        ctx.fillStyle = bubbleGrad;
        
        ctx.beginPath();
        if (hasThreat) {
          // Tehdit altında bozuk, pürüzlü, köşeli bir şekil çiz (Tehdit Algısı)
          const points = 7;
          for (let i = 0; i < points; i++) {
            const angle = (i / points) * Math.PI * 2 + (Date.now() * 0.0005);
            // Genlikte dalgalanma yaparak tehdit hissi uyandırıyoruz
            const offset = Math.sin(angle * 3 + (Date.now() * 0.008)) * 8;
            const px = cx + Math.cos(angle) * (enclaveR + offset);
            const py = cy + Math.sin(angle) * (enclaveR + offset);
            if (i === 0) ctx.moveTo(px, py); else ctx.lineTo(px, py);
          }
          ctx.closePath();
        } else {
          // Güvendeyken dairesel, stabil, akışkan hücre
          ctx.arc(cx, cy, enclaveR, 0, Math.PI * 2);
        }
        ctx.fill();

        // B. Pulsing Neon Hücre Sınır Hattı
        const pulseT = Date.now() * 0.003;
        ctx.strokeStyle = enclaveColor;
        ctx.lineWidth = hasThreat ? 1.8 : 0.8;
        ctx.shadowColor = enclaveColor;
        ctx.shadowBlur = 6 + Math.sin(pulseT) * 4;
        ctx.setLineDash(gID === 1 ? [6, 6] : gID === 3 ? [] : [3, 5]); // Gateway kesikli, DB düz çizgi
        ctx.stroke();
        ctx.setLineDash([]);
        ctx.shadowBlur = 0; // Gölge temizliği

        // C. Soyut Enclave Hücre Etiketi (Premium Cyberpunk HUD)
        const hudFontSize = Math.max(7 / globalScale, 2.2);
        ctx.font = `600 ${hudFontSize}px "Courier New", monospace`;
        ctx.fillStyle = enclaveColor + 'cc';
        ctx.textAlign = 'center';
        ctx.fillText(enclaveLabel, cx, cy - enclaveR - 6);

        ctx.restore();
      }
    }

    // ── NODE RENDER KISMI ──

    // Outer glow
    if (isAct || isSel) {
      const t = Date.now() * 0.005;
      const pulseR = r + 9 + Math.sin(t) * 4;
      const grad = ctx.createRadialGradient(x, y, r, x, y, pulseR);
      grad.addColorStop(0, color + '60');
      grad.addColorStop(1, color + '00');
      ctx.beginPath(); ctx.arc(x, y, pulseR, 0, Math.PI * 2);
      ctx.fillStyle = grad; ctx.fill();

      // Dynamic energy ripple ring that spreads outward
      const rippleR = r + 15 + ((Date.now() * 0.015) % 25);
      const alpha = Math.max(0, 1 - (rippleR - r) / 25) * 0.4;
      ctx.beginPath(); ctx.arc(x, y, rippleR, 0, Math.PI * 2);
      ctx.strokeStyle = color;
      ctx.globalAlpha = alpha;
      ctx.lineWidth = 1.5;
      ctx.stroke();
      ctx.globalAlpha = 1.0; // reset
    }

    // Main circle
    const grad2 = ctx.createRadialGradient(x - r * 0.3, y - r * 0.3, 0, x, y, r);
    grad2.addColorStop(0, color + 'ee');
    grad2.addColorStop(1, color + '99');
    ctx.beginPath(); ctx.arc(x, y, r, 0, Math.PI * 2);
    ctx.fillStyle = grad2; ctx.fill();
    ctx.strokeStyle = isSel ? '#ffffff' : isHov ? '#e2e8f0' : color + '70';
    ctx.lineWidth = isSel ? 2.5 : isHov ? 1.8 : 0.8;
    ctx.stroke();

    // Expand ring
    if (hasKids) {
      ctx.beginPath(); ctx.arc(x, y, r + 4, 0, Math.PI * 2);
      ctx.strokeStyle = isExp ? '#22c55ea0' : '#06b6d4a0';
      ctx.lineWidth = 1.5; ctx.setLineDash([4, 4]); ctx.stroke(); ctx.setLineDash([]);
    }

    // Label (Premium typography & box)
    const label = node.name || node.id;
    const fontSize = Math.max(10.5 / globalScale, 3);
    ctx.font = `${isSel ? '700' : '600'} ${fontSize}px system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif`;
    ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
    const tw = ctx.measureText(label).width;
    const pad = 3;
    ctx.fillStyle = 'rgba(5,10,20,0.82)';
    drawRoundRect(ctx, x - tw / 2 - pad, y + r + 5, tw + pad * 2, fontSize + pad * 2, 3);
    ctx.fill();
    ctx.fillStyle = isSel || isHov ? '#ffffff' : '#cfd8dc';
    ctx.fillText(label, x, y + r + 5 + fontSize / 2 + pad);
  }, [hoveredNode, selectedNode, activeNodeSet, allData, expandedNodes, groupLeaders, visibleData.nodes, alerts]);

  // Link Canvas render döngüsü
  const linkCanvasObject = useCallback((link, ctx, globalScale) => {
    const sx = link.source.x, sy = link.source.y;
    const tx = link.target.x, ty = link.target.y;
    if (!Number.isFinite(sx) || !Number.isFinite(sy) || !Number.isFinite(tx) || !Number.isFinite(ty)) return;

    const active = isActive(link);
    if (!active) {
      // Inactive/Standard links: subtle, sleek line
      ctx.beginPath(); ctx.moveTo(sx, sy); ctx.lineTo(tx, ty);
      ctx.strokeStyle = 'rgba(100, 116, 139, 0.28)';
      ctx.lineWidth = 1.0;
      ctx.stroke();
    } else {
      // Active links: Premium neon glowing laser line
      ctx.beginPath(); ctx.moveTo(sx, sy); ctx.lineTo(tx, ty);
      ctx.strokeStyle = '#22d3ee';
      ctx.lineWidth = 3.5;
      ctx.shadowColor = '#06b6d4';
      ctx.shadowBlur = 14;
      ctx.stroke();
      ctx.shadowBlur = 0; // reset shadow immediately to avoid performance cost

      // Glowing core inner light line
      ctx.beginPath(); ctx.moveTo(sx, sy); ctx.lineTo(tx, ty);
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 1.2;
      ctx.stroke();

      // Energetic flowing traveling particles (energy pulses) with glow
      const t = (Date.now() % 1000) / 1000; // Fast energetic ripple transmission
      const numParticles = 4;
      for (let i = 0; i < numParticles; i++) {
        const p = (t + i / numParticles) % 1;
        const px = sx + (tx - sx) * p;
        const py = sy + (ty - sy) * p;

        ctx.beginPath();
        ctx.arc(px, py, 4.0, 0, Math.PI * 2);
        ctx.fillStyle = '#4ade80';
        ctx.shadowColor = '#4ade80';
        ctx.shadowBlur = 9;
        ctx.fill();
        ctx.shadowBlur = 0; // reset
      }
    }

    // Link label (only when zoomed in enough)
    const label = link.label;
    if (label && globalScale > 1.2) {
      const mx = (sx + tx) / 2, my = (sy + ty) / 2;
      const fontSize = Math.max(7 / globalScale, 2);
      ctx.font = `400 ${fontSize}px system-ui, sans-serif`;
      ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
      const tw = ctx.measureText(label).width;
      // Background
      ctx.fillStyle = active ? 'rgba(5,20,40,0.85)' : 'rgba(15,23,42,0.8)';
      drawRoundRect(ctx, mx - tw / 2 - 2, my - fontSize / 2 - 1.5, tw + 4, fontSize + 3, 2);
      ctx.fill();
      // Text
      ctx.fillStyle = active ? '#22d3ee' : '#94a3b8';
      ctx.fillText(label, mx, my);
    }
  }, [isActive]);

  const flowCount = useMemo(() => {
    const now = Date.now();
    return activeFlows.filter(f => now - f.ts < FLOW_TTL_MS).length;
  }, [activeFlows]);

  return (
    <div ref={containerRef} className="graph-container">
      <div className="graph-hud">
        <div className="graph-hud-title" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
          <div style={{ display: 'flex', alignItems: 'center' }}>
            <span className={`graph-hud-dot ${flowCount > 0 ? 'active' : ''}`} />
            Network Topology
          </div>
          <button
            onClick={handleToggleKafka}
            style={{
              background: kafkaEnabled ? 'rgba(0, 255, 102, 0.2)' : 'rgba(255,255,255,0.03)',
              border: kafkaEnabled ? '1px solid #00ff66' : '1px solid rgba(255,255,255,0.1)',
              color: kafkaEnabled ? '#82ffb0' : '#94a3b8',
              borderRadius: '4px',
              padding: '4px 8px',
              fontSize: '11px',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              fontWeight: '600',
              textShadow: kafkaEnabled ? '0 0 8px rgba(0, 255, 102, 0.4)' : 'none',
              boxShadow: kafkaEnabled ? '0 0 10px rgba(0, 255, 102, 0.15)' : 'none',
              transition: 'all 0.3s ease-in-out',
              userSelect: 'none'
            }}
          >
            <span style={{ 
              display: 'inline-block',
              width: '6px',
              height: '6px',
              borderRadius: '50%',
              background: kafkaEnabled ? '#00ff66' : '#475569',
              boxShadow: kafkaEnabled ? '0 0 8px #00ff66' : 'none',
              transition: 'all 0.3s ease-in-out'
            }} />
            {kafkaEnabled ? 'Kafka Stream: ACTIVE' : 'Kafka Stream: INACTIVE'}
          </button>
        </div>
        <div className="graph-hud-stats">
          <div className="graph-hud-stat">
            <span className="graph-hud-num" style={{color:'#3b82f6'}}>{visibleData.nodes.length}</span>
            <span className="graph-hud-label">NODES</span>
          </div>
          <div className="graph-hud-stat">
            <span className="graph-hud-num" style={{color: flowCount > 0 ? '#4ade80' : '#475569'}}>{flowCount}</span>
            <span className="graph-hud-label">FLOWS</span>
          </div>
          <div className="graph-hud-stat">
            <span className="graph-hud-num" style={{color:'#8b5cf6'}}>{visibleData.links.length}</span>
            <span className="graph-hud-label">LINKS</span>
          </div>
        </div>
      </div>

      <ForceGraph2D
        ref={fgRef}
        width={dims.w}
        height={dims.h}
        graphData={visibleData}
        nodeCanvasObject={nodeCanvasObject}
        nodePointerAreaPaint={(node, color, ctx) => {
          const r = Math.sqrt(node.val || 4) * 5.5 + 4;
          ctx.beginPath(); ctx.arc(node.x, node.y, r, 0, Math.PI * 2);
          ctx.fillStyle = color; ctx.fill();
        }}
        linkCanvasObject={linkCanvasObject}
        onNodeClick={handleClick}
        onNodeHover={n => setHoveredNode(n?.id || null)}
        backgroundColor="rgba(0,0,0,0)"
        enableNodeDrag
        d3AlphaDecay={0.008}
        d3VelocityDecay={0.22}
        warmupTicks={150}
        cooldownTicks={500}
        d3Force="charge"
        d3ForceStrength={-160}
        onEngineStop={() => {
          if (fgRef.current) fgRef.current.zoomToFit(400, 60);
        }}
        dagMode={null}
        linkDirectionalParticles={0}
        nodeRelSize={4}
        onDagError={() => {}}
      />
    </div>
  );
}
