import { createBrowserRouter } from 'react-router-dom'
import { lazy, Suspense, type ComponentType } from 'react'
import Layout from './components/Layout'

function page(loader: () => Promise<{ default: ComponentType }>) {
  const C = lazy(loader)
  return (
    <Suspense fallback={<div className="ev-panel">Loading…</div>}>
      <C />
    </Suspense>
  )
}

export const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      { index: true,           element: page(() => import('./pages/SummaryPage')) },
      { path: 'nodes',         element: page(() => import('./pages/NodesPage')) },
      { path: 'services',      element: page(() => import('./pages/ServicesPage')) },
      { path: 'dns',           element: page(() => import('./pages/DNSPage')) },
      { path: 'peers',         element: page(() => import('./pages/PeersPage')) },
      { path: 'blockchain',    element: page(() => import('./pages/BlockchainPage')) },
    ],
  },
], { basename: '/app' })
