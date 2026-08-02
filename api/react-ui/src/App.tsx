import './styles/base.css'
import Mark from './components/Mark'

export default function App() {
  return (
    <div style={{ padding: 'var(--ev-5)', display: 'flex', gap: 'var(--ev-3)', alignItems: 'center' }}>
      <Mark size={32} />
      <span>edge<span className="slash">/</span>vpn</span>
    </div>
  )
}
