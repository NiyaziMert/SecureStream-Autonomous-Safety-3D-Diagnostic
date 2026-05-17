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

export default function NetworkGraph({ apiKey, apiUrl, activeFlows = [], onNodeSelect }) {
  const [allData, setAllData] = useState({ nodes: [], links: [] });
  const [expandedNodes, setExpandedNodes] = useState(new Set());
  const [hoveredNode, setHoveredNode] = useState(null);
  const [selectedNode, setSelectedNode] = useState(null);
  const fgRef = useRef();
  const containerRef = useRef(null);
  const [dims, setDims] = useState({ w: 800, h: 600 });

  useEffect(() => {
    if (!containerRef.current) return;
    const ro = new ResizeObserver(entries => {
      for (const e of entries) setDims({ w: e.contentRect.width, h: e.contentRect.height });
    });
    ro.observe(containerRef.current);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    const load = () =>
      fetch(`${apiUrl}/topology`, { headers: { 'X-API-Key': apiKey } })
        .then(r => r.json()).then(d => setAllData(d)).catch(() => {});
    load();
    const iv = setInterval(load, 5000);
    return () => clearInterval(iv);
  }, [apiUrl, apiKey]);


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
    const vis = allData.nodes.filter(n => !n.parent || expandedNodes.has(n.parent));
    const ids = new Set(vis.map(n => n.id));
    const links = allData.links.filter(l => {
      const s = typeof l.source === 'object' ? l.source.id : l.source;
      const t = typeof l.target === 'object' ? l.target.id : l.target;
      return ids.has(s) && ids.has(t);
    });
    return { nodes: vis, links };
  }, [allData, expandedNodes]);

  // Configure d3 forces for better spacing
  useEffect(() => {
    if (!fgRef.current) return;
    const fg = fgRef.current;
    fg.d3Force('charge').strength(-400).distanceMax(600);
    fg.d3Force('link').distance(90).strength(0.3);
    fg.d3Force('center').strength(0.05);
    fg.d3ReheatSimulation();
  }, [visibleData]);

  const handleClick = useCallback(node => {
    const hasKids = allData.nodes.some(n => n.parent === node.id);
    if (hasKids) {
      setExpandedNodes(prev => {
        const next = new Set(prev);
        if (next.has(node.id)) {
          const collapse = id => { next.delete(id); allData.nodes.filter(n => n.parent === id).forEach(c => collapse(c.id)); };
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

  const nodeCanvasObject = useCallback((node, ctx, globalScale) => {
    const x = node.x, y = node.y;
    if (!Number.isFinite(x) || !Number.isFinite(y)) return;

    const color = getNodeColor(node);
    const isHov = hoveredNode === node.id;
    const isSel = selectedNode === node.id;
    const isAct = activeNodeSet.has(node.id);
    const hasKids = allData.nodes.some(n => n.parent === node.id);
    const isExp = expandedNodes.has(node.id);
    const baseR = Math.sqrt(node.val || 4) * 2.5;
    const r = baseR * (isHov ? 1.3 : isSel ? 1.2 : 1);

    // Outer glow
    if (isAct || isSel) {
      const t = Date.now() * 0.003;
      const pulseR = r + 6 + Math.sin(t) * 3;
      const grad = ctx.createRadialGradient(x, y, r, x, y, pulseR);
      grad.addColorStop(0, color + '40');
      grad.addColorStop(1, color + '00');
      ctx.beginPath(); ctx.arc(x, y, pulseR, 0, Math.PI * 2);
      ctx.fillStyle = grad; ctx.fill();
    }

    // Main circle
    const grad2 = ctx.createRadialGradient(x - r * 0.3, y - r * 0.3, 0, x, y, r);
    grad2.addColorStop(0, color + 'ee');
    grad2.addColorStop(1, color + '88');
    ctx.beginPath(); ctx.arc(x, y, r, 0, Math.PI * 2);
    ctx.fillStyle = grad2; ctx.fill();
    ctx.strokeStyle = isSel ? '#ffffff' : isHov ? '#e2e8f0' : color + '60';
    ctx.lineWidth = isSel ? 2 : isHov ? 1.5 : 0.5;
    ctx.stroke();

    // Expand ring
    if (hasKids) {
      ctx.beginPath(); ctx.arc(x, y, r + 3, 0, Math.PI * 2);
      ctx.strokeStyle = isExp ? '#22c55e90' : '#f59e0b90';
      ctx.lineWidth = 1.5; ctx.setLineDash([3, 3]); ctx.stroke(); ctx.setLineDash([]);
    }

    // Label
    const label = node.name || node.id;
    const fontSize = Math.max(10 / globalScale, 2.5);
    ctx.font = `${isSel ? '600' : '500'} ${fontSize}px Inter, sans-serif`;
    ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
    const tw = ctx.measureText(label).width;
    const pad = 2;
    ctx.fillStyle = 'rgba(5,10,20,0.75)';
    drawRoundRect(ctx, x - tw / 2 - pad, y + r + 4, tw + pad * 2, fontSize + pad * 2, 2);
    ctx.fill();
    ctx.fillStyle = isSel || isHov ? '#ffffff' : '#b0bec5';
    ctx.fillText(label, x, y + r + 4 + fontSize / 2 + pad);
  }, [hoveredNode, selectedNode, activeNodeSet, allData, expandedNodes]);

  const linkCanvasObject = useCallback((link, ctx, globalScale) => {
    const sx = link.source.x, sy = link.source.y;
    const tx = link.target.x, ty = link.target.y;
    if (!Number.isFinite(sx) || !Number.isFinite(sy) || !Number.isFinite(tx) || !Number.isFinite(ty)) return;

    const active = isActive(link);
    // Line
    ctx.beginPath(); ctx.moveTo(sx, sy); ctx.lineTo(tx, ty);
    ctx.strokeStyle = active ? '#22d3ee88' : 'rgba(80,100,160,0.12)';
    ctx.lineWidth = active ? 2 : 0.4;
    ctx.stroke();

    // Animated particles
    if (active) {
      const t = (Date.now() % 2000) / 2000;
      for (let i = 0; i < 3; i++) {
        const p = (t + i / 3) % 1;
        ctx.beginPath();
        ctx.arc(sx + (tx - sx) * p, sy + (ty - sy) * p, 2, 0, Math.PI * 2);
        ctx.fillStyle = '#4ade80'; ctx.fill();
      }
    }

    // Link label (only when zoomed in enough)
    const label = link.label;
    if (label && globalScale > 1.2) {
      const mx = (sx + tx) / 2, my = (sy + ty) / 2;
      const fontSize = Math.max(7 / globalScale, 2);
      ctx.font = `400 ${fontSize}px Inter, sans-serif`;
      ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
      const tw = ctx.measureText(label).width;
      // Background
      ctx.fillStyle = active ? 'rgba(5,20,40,0.85)' : 'rgba(5,10,20,0.7)';
      drawRoundRect(ctx, mx - tw / 2 - 2, my - fontSize / 2 - 1.5, tw + 4, fontSize + 3, 2);
      ctx.fill();
      // Text
      ctx.fillStyle = active ? '#22d3ee' : '#64748b';
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
        <div className="graph-hud-title">
          <span className={`graph-hud-dot ${flowCount > 0 ? 'active' : ''}`} />
          Network Topology
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
          const r = Math.sqrt(node.val || 4) * 3;
          ctx.beginPath(); ctx.arc(node.x, node.y, r, 0, Math.PI * 2);
          ctx.fillStyle = color; ctx.fill();
        }}
        linkCanvasObject={linkCanvasObject}
        onNodeClick={handleClick}
        onNodeHover={n => setHoveredNode(n?.id || null)}
        backgroundColor="rgba(0,0,0,0)"
        enableNodeDrag
        d3AlphaDecay={0.01}
        d3VelocityDecay={0.25}
        warmupTicks={120}
        cooldownTicks={400}
        d3Force="charge"
        d3ForceStrength={-350}
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
