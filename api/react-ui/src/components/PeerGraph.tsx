import { useEffect, useRef } from 'react'
import type { PeerRow } from '../types/api'

type Props = { peers: PeerRow[]; selfId: string }

/**
 * The peers the ring actually plots: ones this node has a relationship with.
 *
 * A peerstore entry is an address the DHT handed us, not a connection. On a
 * real node there are hundreds to thousands of them, and drawing a spoke to
 * each would both bury the graph under a solid disc and claim a connectivity
 * this node does not have. Exported so the page can name the excluded count
 * instead of quietly dropping it.
 */
export function plottedPeers(peers: PeerRow[]): PeerRow[] {
  return peers.filter((p) => p.online || p.known)
}

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
  const plotted = plottedPeers(peers)
  const excluded = peers.length - plotted.length
  const peersRef = useRef(plotted)
  peersRef.current = plotted
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

    // Current CSS size and device pixel ratio of the canvas. Assigning
    // canvas.width resets the bitmap, so it must happen only when one of these
    // actually changed — otherwise every frame wipes itself and the clearRect
    // below is dead code. Measured once here, then kept current by the
    // ResizeObserver: this canvas sits above a table of ~1000 rows that
    // re-renders on every poll, and a getBoundingClientRect per frame would
    // force a synchronous layout each time.
    let cssW = 0, cssH = 0, ratio = 0

    function applySize(w: number, h: number, r: number) {
      cssW = w; cssH = h; ratio = r
      canvas!.width = Math.round(w * r)
      canvas!.height = Math.round(h * r)
      ctx!.setTransform(r, 0, 0, r, 0, 0)
    }

    const dpr = () => Math.min(window.devicePixelRatio || 1, 2)
    const first = canvas.getBoundingClientRect()
    applySize(first.width, first.height, dpr())

    function draw(t: number) {
      // devicePixelRatio is free to read and forces no layout, so zoom changes
      // — which leave the CSS size alone — are still picked up.
      const r = dpr()
      if (r !== ratio) applySize(cssW, cssH, r)

      const w = cssW, h = cssH
      const cx = w / 2, cy = h / 2
      const radius = Math.min(w, h) / 2 - 26
      ctx!.clearRect(0, 0, w, h)

      const list = peersRef.current
      // Reduced, not Math.max(1, ...list): spreading an array as arguments
      // throws RangeError once it gets large, which would kill the loop for
      // good on a node with a big peer set.
      const maxRate = list.reduce((m, p) => Math.max(m, p.rateIn + p.rateOut), 1)

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

    // Stop burning frames when the graph scrolls out of view. Read the *last*
    // entry, not the first: a batch can carry more than one crossing, and
    // acting on the stale one would either stall the loop while visible or
    // leave it running off-screen.
    const io = new IntersectionObserver((entries) => {
      const entry = entries[entries.length - 1]
      if (entry.isIntersecting) {
        if (!running) { running = true; raf = requestAnimationFrame(draw) }
      } else {
        running = false
        cancelAnimationFrame(raf)
      }
    }, { threshold: 0.05 })
    io.observe(canvas)

    // Replaces a window resize listener: this fires for element size changes
    // from any cause, and it is the only thing that touches the bitmap size.
    const ro = new ResizeObserver((entries) => {
      const box = entries[entries.length - 1].contentRect
      if (box.width === cssW && box.height === cssH) return
      applySize(box.width, box.height, ratio)
      if (reduced) draw(0)
    })
    ro.observe(canvas)

    if (reduced) redrawRef.current = () => draw(0)

    return () => {
      running = false
      cancelAnimationFrame(raf)
      io.disconnect()
      ro.disconnect()
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
      aria-label={
        `Network graph: this node and the ${plotted.length} ` +
        `${plotted.length === 1 ? 'peer' : 'peers'} it is connected to or shares a ` +
        'VPN address with.' +
        (excluded > 0
          ? ` ${excluded} address-book ${excluded === 1 ? 'entry is' : 'entries are'} not drawn.`
          : '') +
        ' The table below lists every peer.'
      }
      style={{ display: 'block', width: '100%', height: '260px' }}
      data-self={selfId}
    />
  )
}
