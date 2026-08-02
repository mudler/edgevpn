import { NavLink, Outlet } from 'react-router-dom'
import Mark from './Mark'
import './Layout.css'

const ROUTES = [
  { to: '/', label: 'Summary', end: true },
  { to: '/nodes', label: 'Nodes', end: false },
  { to: '/services', label: 'Services', end: false },
  { to: '/dns', label: 'DNS', end: false },
  { to: '/peers', label: 'Peers', end: false },
  { to: '/blockchain', label: 'Ledger', end: false },
]

export default function Layout() {
  return (
    <div className="ev-shell">
      <header className="ev-header">
        <div className="ev-brand">
          <Mark size={22} />
          <span>edge<span className="slash">/</span>vpn</span>
        </div>
        <nav className="ev-nav">
          {ROUTES.map((r) => (
            <NavLink key={r.to} to={r.to} end={r.end}
                     className={({ isActive }) => (isActive ? 'active' : undefined)}>
              {r.label}
            </NavLink>
          ))}
        </nav>
      </header>
      <main className="ev-main">
        <Outlet />
      </main>
    </div>
  )
}
