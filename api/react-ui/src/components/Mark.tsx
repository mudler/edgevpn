type MarkProps = {
  size?: number
  title?: string
}

export default function Mark({ size = 24, title = 'EdgeVPN' }: MarkProps) {
  // Stroke widths scale up at small sizes so the mark stays legible
  // in a favicon or a 16px nav slot.
  const heavy = size <= 20
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 64 64"
      role="img"
      aria-label={title}
      style={{ flex: 'none' }}
    >
      <circle cx="13" cy="49" r="7" fill="none"
              stroke="var(--ev-muted)" strokeWidth={heavy ? 6 : 3} />
      <circle cx="51" cy="15" r="7" fill="none"
              stroke="var(--ev-muted)" strokeWidth={heavy ? 6 : 3} />
      <path d="M20 42 L44 22" stroke="var(--ev-signal)"
            strokeWidth={heavy ? 8 : 5} strokeLinecap="round" />
    </svg>
  )
}
