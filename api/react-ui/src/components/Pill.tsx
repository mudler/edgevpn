import type { ReactNode } from 'react'

type Tone = 'ok' | 'warn' | 'crit'
type PillProps = { tone: Tone; children: ReactNode }

export default function Pill({ tone, children }: PillProps) {
  return <span className={`ev-pill ev-pill--${tone}`}>{children}</span>
}
