import { useEffect, useRef } from 'react'
import type { PeerRow } from '../pages/PeersPage'

type Props = { peers: PeerRow[]; selfId: string }

/**
 * Ego graph: this node at the centre, its peers on a ring.
 *
 * Edge thickness encodes real per-peer bandwidth from /api/metrics/peer.
 * Edges between other peers are deliberately absent — no endpoint exposes
 * that topology, and inventing it would be a lie about the network.
 *
 * The peers table below is the accessible equivalent; this canvas is
 * additive and is never the only way to read the data.
 */
export default function PeerGraph({ peers, selfId }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const peersRef = useRef(peers)
  peersRef.current = peers
  // Set only under reduced motion, where a single frame is drawn instead of a
  // loop and therefore has to be re-issued by hand when the data changes.
  const redrawRef = useRef<(() => void) | null>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    let raf = 0
    let running = true

    // Canvas cannot read CSS custom properties directly, so resolve them
    // once here. No literal fallbacks: tokens.css is imported by base.css
    // before this ever mounts, and a silent hardcoded fallback would let
    // the canvas drift out of the design system unnoticed.
    const css = getComputedStyle(document.documentElement)
    const colSignal = css.getPropertyValue('--ev-signal').trim()
    const colMuted = css.getPropertyValue('--ev-muted').trim()
    const colRule = css.getPropertyValue('--ev-rule').trim()
    const colOk = css.getPropertyValue('--ev-ok').trim()

    function resize() {
      const dpr = Math.min(window.devicePixelRatio || 1, 2)
      const rect = canvas!.getBoundingClientRect()
      canvas!.width = Math.round(rect.width * dpr)
      canvas!.height = Math.round(rect.height * dpr)
      ctx!.setTransform(dpr, 0, 0, dpr, 0, 0)
      return rect
    }

    function draw(t: number) {
      const rect = resize()
      const w = rect.width, h = rect.height
      const cx = w / 2, cy = h / 2
      const radius = Math.min(w, h) / 2 - 26
      ctx!.clearRect(0, 0, w, h)

      const list = peersRef.current
      const maxRate = Math.max(1, ...list.map((p) => p.rateIn + p.rateOut))

      list.forEach((p, i) => {
        const angle = (i / Math.max(1, list.length)) * Math.PI * 2 - Math.PI / 2
        const px = cx + Math.cos(angle) * radius
        const py = cy + Math.sin(angle) * radius

        // Edge: width from real traffic, colour from liveness.
        const share = (p.rateIn + p.rateOut) / maxRate
        ctx!.strokeStyle = p.online ? colMuted : colRule
        ctx!.lineWidth = 0.6 + share * 3
        ctx!.beginPath()
        ctx!.moveTo(cx, cy)
        ctx!.lineTo(px, py)
        ctx!.stroke()

        // A pulse travelling outward, only where traffic is actually flowing.
        if (!reduced && share > 0.02) {
          const phase = ((t / 1400) + i * 0.13) % 1
          ctx!.fillStyle = colSignal
          ctx!.beginPath()
          ctx!.arc(cx + (px - cx) * phase, cy + (py - cy) * phase, 2, 0, Math.PI * 2)
          ctx!.fill()
        }

        ctx!.fillStyle = p.online ? colOk : colRule
        ctx!.beginPath()
        ctx!.arc(px, py, 4.5, 0, Math.PI * 2)
        ctx!.fill()
      })

      // This node, last so it sits on top.
      ctx!.fillStyle = colSignal
      ctx!.beginPath()
      ctx!.arc(cx, cy, 7, 0, Math.PI * 2)
      ctx!.fill()

      if (running && !reduced) raf = requestAnimationFrame(draw)
    }

    raf = requestAnimationFrame(draw)

    // Stop burning frames when the graph scrolls out of view.
    const io = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) {
        if (!running) { running = true; raf = requestAnimationFrame(draw) }
      } else {
        running = false
        cancelAnimationFrame(raf)
      }
    }, { threshold: 0.05 })
    io.observe(canvas)

    const onResize = () => { if (reduced) draw(0) }
    window.addEventListener('resize', onResize)
    if (reduced) redrawRef.current = () => draw(0)

    return () => {
      running = false
      cancelAnimationFrame(raf)
      io.disconnect()
      window.removeEventListener('resize', onResize)
      redrawRef.current = null
    }
  }, [])

  // Under reduced motion exactly one frame is ever drawn, and it lands on mount
  // — before the first poll returns, when the peer list is still empty. Without
  // this the graph would be a bare centre dot forever for anyone who asked for
  // less motion, which is precisely the user this is supposed to serve.
  // Declared after the effect above so redrawRef is set by the time it runs.
  useEffect(() => { redrawRef.current?.() }, [peers])

  return (
    <canvas
      ref={canvasRef}
      role="img"
      aria-label={`Network graph: this node connected to ${peers.length} peers. The table below lists them.`}
      style={{ display: 'block', width: '100%', height: '260px' }}
      data-self={selfId}
    />
  )
}
