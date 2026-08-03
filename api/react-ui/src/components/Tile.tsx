type TileProps = { label: string; value: string | number }

export default function Tile({ label, value }: TileProps) {
  return (
    <div className="ev-tile">
      <span className="ev-tile-k">{label}</span>
      <span className="ev-tile-v tabular">{value}</span>
    </div>
  )
}
