import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import ForceGraph3D from 'react-force-graph-3d';
import * as THREE from 'three';
import SpriteText from 'three-spritetext';

const FLOW_TTL_MS = 2500; // bir flow olayı kaç ms aktif kalacak

export default function Topology3D({ apiKey, apiUrl, activeFlows = [] }) {
  const [allData, setAllData]         = useState({ nodes: [], links: [] });
  const [expandedNodes, setExpandedNodes] = useState(new Set());
  const fgRef = useRef();

  // ── Topolojiyi çek (5 sn'de bir yenile) ──────────────────
  useEffect(() => {
    const load = () =>
      fetch(`${apiUrl}/topology`, { headers: { 'X-API-Key': apiKey } })
        .then(r => r.json())
        .then(d => setAllData(d))
        .catch(() => {});
    load();
    const iv = setInterval(load, 5000);
    return () => clearInterval(iv);
  }, [apiUrl, apiKey]);

  // ── Kamera ilk yüklendiğinde geri çekil ──────────────────
  useEffect(() => {
    if (fgRef.current && allData.nodes.length > 0) {
      fgRef.current.cameraPosition({ x: 0, y: 0, z: 900 }, { x: 0, y: 0, z: 0 }, 2000);
    }
  }, [allData.nodes.length > 0]); // eslint-disable-line

  // ── Aktif link seti (flow olaylarından) ──────────────────
  const activeLinkSet = useMemo(() => {
    const now = Date.now();
    const s = new Set();
    activeFlows
      .filter(f => now - f.ts < FLOW_TTL_MS)
      .forEach(f => {
        s.add(`${f.source}|||${f.target}`);
        s.add(`${f.target}|||${f.source}`);
      });
    return s;
  }, [activeFlows]);

  const isLinkActive = useCallback((link) => {
    const sId = typeof link.source === 'object' ? link.source.id : link.source;
    const tId = typeof link.target === 'object' ? link.target.id : link.target;
    return activeLinkSet.has(`${sId}|||${tId}`) || activeLinkSet.has(`${tId}|||${sId}`);
  }, [activeLinkSet]);

  // ── Görünür düğüm/bağlantı filtresi ─────────────────────
  const visibleData = useMemo(() => {
    const visible = allData.nodes.filter(n => !n.parent || expandedNodes.has(n.parent));
    const visibleIds = new Set(visible.map(n => n.id));
    const links = allData.links.filter(l => {
      const s = typeof l.source === 'object' ? l.source.id : l.source;
      const t = typeof l.target === 'object' ? l.target.id : l.target;
      return visibleIds.has(s) && visibleIds.has(t);
    });
    return { nodes: visible, links };
  }, [allData, expandedNodes]);

  // ── Node tıklandığında expand/collapse + zoom ─────────────
  const handleNodeClick = useCallback(node => {
    const hasKids = allData.nodes.some(n => n.parent === node.id);
    if (hasKids) {
      setExpandedNodes(prev => {
        const next = new Set(prev);
        if (next.has(node.id)) {
          // Kapat — torunları da kapat
          const collapse = (id) => {
            next.delete(id);
            allData.nodes.filter(n => n.parent === id).forEach(c => collapse(c.id));
          };
          collapse(node.id);
        } else {
          next.add(node.id);
        }
        return next;
      });
    }
    // Kameraya node'a doğru zoom yap
    if (fgRef.current) {
      const dist = 180;
      const mag = Math.hypot(node.x || 0.01, node.y || 0.01, node.z || 0.01);
      const ratio = 1 + dist / mag;
      fgRef.current.cameraPosition(
        { x: node.x * ratio, y: node.y * ratio, z: node.z * ratio },
        node, 1200
      );
    }
  }, [allData]);

  // ── Özel 3D node nesnesi ─────────────────────────────────
  const getNodeThreeObject = useCallback(node => {
    const hasKids   = allData.nodes.some(n => n.parent === node.id);
    const isExpanded = expandedNodes.has(node.id);
    const color     = node.color || '#ffffff';
    const radius    = Math.sqrt(node.val || 4) * 2.5;

    const group = new THREE.Group();

    // Küre
    const geo = new THREE.SphereGeometry(radius, 20, 20);
    const mat = new THREE.MeshPhongMaterial({
      color,
      emissive: color,
      emissiveIntensity: 0.35,
      shininess: 80,
      transparent: true,
      opacity: 0.92,
    });
    group.add(new THREE.Mesh(geo, mat));

    // İsim etiketi
    const label = new SpriteText(node.name || node.id);
    label.color       = '#e2e8f0';
    label.textHeight  = Math.max(radius * 0.75, 3.5);
    label.position.y  = radius + 5;
    label.backgroundColor = 'rgba(0,0,0,0.45)';
    label.padding     = 1.5;
    label.borderRadius = 3;
    group.add(label);

    // Genişletilebilir node halkası
    if (hasKids) {
      const ringGeo = new THREE.TorusGeometry(radius + 2.5, 0.6, 8, 48);
      const ringMat = new THREE.MeshBasicMaterial({
        color: isExpanded ? '#22c55e' : '#f59e0b',
        transparent: true,
        opacity: 0.85,
      });
      group.add(new THREE.Mesh(ringGeo, ringMat));

      // Ok işareti
      const arrowLabel = new SpriteText(isExpanded ? '▲ kapat' : '▼ aç');
      arrowLabel.color      = isExpanded ? '#22c55e' : '#f59e0b';
      arrowLabel.textHeight = 3;
      arrowLabel.position.y = -(radius + 6);
      group.add(arrowLabel);
    }

    return group;
  }, [allData, expandedNodes]);

  // ── İstatistik ───────────────────────────────────────────
  const activeFlowCount = useMemo(() => {
    const now = Date.now();
    return activeFlows.filter(f => now - f.ts < FLOW_TTL_MS).length;
  }, [activeFlows]);

  const visibleNodeCount  = visibleData.nodes.length;
  const totalNodeCount    = allData.nodes.length;

  return (
    <div style={{ width: '100%', height: '100%', minHeight: 500, background: '#000212', borderRadius: 12, overflow: 'hidden', position: 'relative' }}>

      {/* ── Legend / Bilgi Paneli ─────────────────────────── */}
      <div style={{
        position: 'absolute', top: 12, left: 12, zIndex: 10,
        background: 'rgba(2,6,23,0.82)', backdropFilter: 'blur(10px)',
        border: '1px solid rgba(99,102,241,0.35)',
        borderRadius: 10, padding: '10px 14px', fontSize: 11, color: '#94a3b8',
        maxWidth: 240, lineHeight: 1.6,
      }}>
        <div style={{ fontWeight: 700, fontSize: 12, color: '#e2e8f0', marginBottom: 6 }}>
          Acme E-Commerce Ltd.
        </div>
        <div style={{ display: 'flex', gap: 6, alignItems: 'center', marginBottom: 3 }}>
          <span style={{ color: '#f59e0b', fontSize: 14 }}>◎</span>
          <span>Sarı halka → tıkla, microservisleri gör</span>
        </div>
        <div style={{ display: 'flex', gap: 6, alignItems: 'center', marginBottom: 3 }}>
          <span style={{ color: '#22c55e', fontSize: 14 }}>◎</span>
          <span>Yeşil halka → açık, tekrar tıkla kapat</span>
        </div>
        <div style={{ display: 'flex', gap: 6, alignItems: 'center', marginBottom: 6 }}>
          <span style={{ width: 20, height: 2, background: 'linear-gradient(90deg,#4ade80,#22d3ee)', display: 'inline-block', borderRadius: 2 }}></span>
          <span>Aktif veri akışı (impulse)</span>
        </div>
        <div style={{ borderTop: '1px solid rgba(255,255,255,0.08)', paddingTop: 6, display: 'flex', justifyContent: 'space-between' }}>
          <span>Node: {visibleNodeCount}/{totalNodeCount}</span>
          <span style={{ color: activeFlowCount > 0 ? '#4ade80' : '#64748b' }}>
            {activeFlowCount > 0 ? `⚡ ${activeFlowCount} aktif akış` : '● bekleniyor'}
          </span>
        </div>
      </div>

      {/* ── ForceGraph3D ──────────────────────────────────── */}
      <ForceGraph3D
        ref={fgRef}
        graphData={visibleData}

        // ── Node görünümü ──
        nodeThreeObject={getNodeThreeObject}
        nodeThreeObjectExtend={false}
        onNodeClick={handleNodeClick}

        // ── Link görünümü ──
        linkWidth={link => isLinkActive(link) ? 2.5 : 0.4}
        linkOpacity={link => isLinkActive(link) ? 0.9 : 0.35}
        linkColor={link => {
          if (isLinkActive(link)) return '#22d3ee';
          const sNode = typeof link.source === 'object'
            ? link.source
            : allData.nodes.find(n => n.id === link.source);
          return sNode ? sNode.color + '88' : '#ffffff44';
        }}

        // ── Impulse partiküller ──
        linkDirectionalParticles={link => isLinkActive(link) ? 10 : 2}
        linkDirectionalParticleWidth={link => isLinkActive(link) ? 4 : 1.5}
        linkDirectionalParticleSpeed={link => isLinkActive(link) ? 0.03 : 0.006}
        linkDirectionalParticleColor={link => {
          if (isLinkActive(link)) return '#4ade80';
          const tNode = typeof link.target === 'object'
            ? link.target
            : allData.nodes.find(n => n.id === link.target);
          return tNode ? tNode.color : '#ffffff';
        }}

        // ── Fizik & kamera ──
        backgroundColor="#000212"
        enableNodeDrag
        d3AlphaDecay={0.015}
        d3VelocityDecay={0.25}
      />
    </div>
  );
}
