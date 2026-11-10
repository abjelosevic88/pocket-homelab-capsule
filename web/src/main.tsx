import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import './styles/base.css'
import './styles/capsule.css'
import './styles/app.css'
import './styles/settings.css'
import { AppProvider, useApp } from './lib/app-context'
import { Loading, ToastProvider } from './components/ui'
import Layout from './components/Layout'
import { ErrorBoundary } from './components/ErrorBoundary'
import Login from './pages/Login'
import Home from './pages/Home'
import NewCapsule from './pages/NewCapsule'
import CapsulePage from './pages/CapsulePage'
import Opening from './pages/Opening'
import Predictions from './pages/Predictions'
import Yearly from './pages/Yearly'
import { GivePage, RecipientPage } from './pages/Public'
import Settings from './pages/Settings'
import { DataProvider } from './lib/data'

function Gate({ children }: { children: React.ReactNode }) {
  const { status, user } = useApp()
  const loc = useLocation()
  if (!status) return <div className="boot"><img src="/favicon.svg" alt="" /><Loading /></div>
  if (status.needs_setup) return <Navigate to="/setup" replace />
  if (!user) return <Navigate to="/login" replace state={{ from: loc.pathname + loc.search }} />
  return <>{children}</>
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login mode="login" />} />
      <Route path="/setup" element={<Login mode="setup" />} />
      <Route path="/o/:key" element={<ErrorBoundary where="capsule"><RecipientPage /></ErrorBoundary>} />
      <Route path="/give/:key" element={<ErrorBoundary where="capsule"><GivePage /></ErrorBoundary>} />
      <Route path="/settings/:tab?" element={<Gate><ErrorBoundary where="settings"><DataProvider><Settings /></DataProvider></ErrorBoundary></Gate>} />
      <Route element={<Gate><ErrorBoundary where="capsule"><Layout /></ErrorBoundary></Gate>}>
        <Route path="/" element={<Home />} />
        <Route path="/new" element={<NewCapsule />} />
        <Route path="/c/:id" element={<CapsulePage />} />
        <Route path="/c/:id/open" element={<Opening />} />
        <Route path="/predictions" element={<Predictions />} />
        <Route path="/yearly" element={<Yearly />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <ErrorBoundary where="app"><ToastProvider><AppProvider><App /></AppProvider></ToastProvider></ErrorBoundary>
    </BrowserRouter>
  </React.StrictMode>,
)
